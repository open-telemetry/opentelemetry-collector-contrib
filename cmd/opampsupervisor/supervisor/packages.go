package supervisor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

var (
	// agentPackageKey is the key used for the top-level package (the agent).
	// According to the spec, an empty key may be used if there is only one top-level package.
	// https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#packages
	agentPackageKey = ""

	packagesStateFileName     = "package-state.yaml"
	lastPackageStatusFileName = "last-reported-package-statuses.proto"
)

type packageState struct {
	allPackagesHash []byte `yaml:"all_packages_hash"`
}

type packageMetadata struct {
	PackageType int32
	Hash        []byte
	Version     string
}

// packageManager manages the persistent state of downloadable packages.
// It persists the packages to the file system.
// Currently, it only allows for a single top-level package containing the agent
// to be received.
type packageManager struct {
	packageState    *packageState
	topLevelHash    []byte
	topLevelVersion string

	storageDir string
	agentPath  string
	checkOpts  *cosign.CheckOpts
	am         agentManager
}

type agentManager interface {
	stopAgentProcess(context.Context)
	startAgentProcess()
}

func newPackageManager(agentPath, storageDir, agentVersion string, signatureOpts config.AgentSignature, am agentManager) (*packageManager, error) {
	// Read actual hash of the on-disk agent
	f, err := os.Open(agentPath)
	if err != nil {
		return nil, fmt.Errorf("open agent: %w", err)
	}

	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("compute agent hash: %w", err)
	}
	agentHash := h.Sum(nil)

	// Load persisted package state, if it exists
	packageStatePath := filepath.Join(storageDir, "packageStatuses.yaml")
	state, err := loadPackageState(packageStatePath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		// initialize default state
		state = &packageState{}
	case err != nil:
		return nil, fmt.Errorf("load package state: %w", err)
	}

	checkOpts, err := createCosignCheckOpts(signatureOpts)
	if err != nil {
		return nil, fmt.Errorf("create signature verification options: %w", err)
	}

	return &packageManager{
		packageState:    state,
		topLevelHash:    agentHash,
		topLevelVersion: agentVersion,
		storageDir:      storageDir,
		agentPath:       agentPath,
		checkOpts:       checkOpts,
		am:              am,
	}, nil
}

func (p packageManager) AllPackagesHash() ([]byte, error) {
	return p.packageState.allPackagesHash, nil
}

func (p packageManager) SetAllPackagesHash(hash []byte) error {
	p.packageState.allPackagesHash = hash
	return savePackageState(p.packagesStatusPath(), p.packageState)
}

func (packageManager) Packages() ([]string, error) {
	return []string{agentPackageKey}, nil
}

func (p packageManager) PackageState(packageName string) (state types.PackageState, err error) {
	if packageName != agentPackageKey {
		return types.PackageState{
			Exists:  true,
			Type:    protobufs.PackageType_PackageType_TopLevel,
			Hash:    p.topLevelHash,
			Version: p.topLevelVersion,
		}, nil
	}

	return types.PackageState{
		Exists: false,
	}, nil
}

func (p *packageManager) SetPackageState(packageName string, state types.PackageState) error {
	if packageName != agentPackageKey {
		return fmt.Errorf("package %q does not exist", packageName)
	}

	if !state.Exists {
		return fmt.Errorf("agent package must be marked as existing")
	}

	if state.Type != protobufs.PackageType_PackageType_TopLevel {
		return fmt.Errorf("agent package must be marked as top level")
	}

	p.topLevelHash = state.Hash
	p.topLevelVersion = state.Version

	return nil
}

func (packageManager) CreatePackage(packageName string, typ protobufs.PackageType) error {
	if packageName != agentPackageKey {
		return fmt.Errorf("only agent package is supported")
	}

	return fmt.Errorf("agent package already exists")
}

func (p *packageManager) FileContentHash(packageName string) ([]byte, error) {
	if packageName != agentPackageKey {
		return nil, nil
	}

	return p.topLevelHash, nil
}

func (p *packageManager) UpdateContent(ctx context.Context, packageName string, data io.Reader, contentHash, signature []byte) error {
	if packageName != agentPackageKey {
		return fmt.Errorf("package does not exist")
	}

	by, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("read package bytes: %w", err)
	}

	if err := verifyPackageIntegrity(by, contentHash); err != nil {
		return fmt.Errorf("could not verify package integrity: %w", err)
	}

	b64Cert, b64Signature, err := parsePackageSignature(signature)
	if err != nil {
		return fmt.Errorf("could not parse package signature: %w", err)
	}

	if err := verifyPackageSignature(ctx, p.checkOpts, by, b64Cert, b64Signature); err != nil {
		return fmt.Errorf("could not verify package signature: %w", err)
	}

	// overwrite agent process
	p.am.stopAgentProcess(ctx)
	// We always want to start the agent process again, even if we fail to write the agent file
	defer p.am.startAgentProcess()

	// Create a backup in case we fail to write the agent
	agentBackupPath := filepath.Join(p.storageDir, "collector.bak")
	err = os.WriteFile(agentBackupPath, by, 0600)
	if err != nil {
		return fmt.Errorf("create agent backup: %w", err)
	}

	f, err := os.OpenFile(p.agentPath, os.O_WRONLY|os.O_TRUNC, 0700)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, bytes.NewBuffer(by)); err != nil {
		restoreErr := restoreBackup(agentBackupPath, p.agentPath)
		return errors.Join(fmt.Errorf("write package to file: %w", err), restoreErr)
	}

	return nil
}

func (p *packageManager) DeletePackage(packageName string) error {
	if packageName != agentPackageKey {
		// We only take the agent package, so the package already doesn't exist.
		return nil
	}

	// We will never delete the agent package.
	return errors.New("cannot delete top-level package")
}

func (p *packageManager) LastReportedStatuses() (*protobufs.PackageStatuses, error) {
	// TODO: What to do if no package status exists
	lastStatusBytes, err := os.ReadFile(p.lastPackageStatusPath())
	if err != nil {
		return nil, fmt.Errorf("read last package statuses: %w", err)
	}

	var ret protobufs.PackageStatuses
	err = proto.Unmarshal(lastStatusBytes, &ret)
	if err != nil {
		return nil, fmt.Errorf("unmarshal last package statuses: %w", err)
	}

	return &ret, nil
}

func (p packageManager) SetLastReportedStatuses(statuses *protobufs.PackageStatuses) error {
	lastStatusBytes, err := proto.Marshal(statuses)
	if err != nil {
		return fmt.Errorf("marshal statuses: %w", err)
	}

	err = os.WriteFile(p.lastPackageStatusPath(), lastStatusBytes, 0600)
	if err != nil {
		return fmt.Errorf("write package statues: %w", err)
	}

	return nil
}

func (p *packageManager) lastPackageStatusPath() string {
	return filepath.Join(p.storageDir, lastPackageStatusFileName)
}

func (p *packageManager) packagesStatusPath() string {
	return filepath.Join(p.storageDir, packagesStateFileName)
}

func loadPackageState(path string) (*packageState, error) {
	by, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var pkgState packageState
	if err := yaml.Unmarshal(by, &pkgState); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}

	return &pkgState, nil
}

func savePackageState(path string, ps *packageState) error {
	by, err := yaml.Marshal(ps)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(by); err != nil {
		return fmt.Errorf("write package state: %w", err)
	}

	return nil
}

func verifyPackageIntegrity(packageBytes, expectedHash []byte) error {
	actualHash := sha256.Sum256(packageBytes)
	if bytes.Equal(actualHash[:], expectedHash) {
		return errors.New("invalid hash for package")
	}

	return nil
}

func parsePackageSignature(signature []byte) (b64Cert, b64Signature []byte, err error) {
	splitSignature := bytes.SplitN(signature, []byte(" "), 2)
	if len(splitSignature) != 2 {
		return nil, nil, fmt.Errorf("signature must be formatted as a space separated cert and signature")
	}

	return splitSignature[0], splitSignature[1], nil
}

// sig is the decoded signature of
func verifyPackageSignature(ctx context.Context, checkOpts *cosign.CheckOpts, packageBytes, b64Cert, b64Signature []byte) error {
	decodedCert, err := base64.StdEncoding.AppendDecode(nil, b64Cert)
	if err != nil {
		return fmt.Errorf("b64 decode cert: %w", err)
	}

	ociSig, err := static.NewSignature(packageBytes, string(b64Signature), static.WithCertChain(decodedCert, nil))
	if err != nil {
		return fmt.Errorf("create signature: %w", err)
	}

	_, err = cosign.VerifyBlobSignature(ctx, ociSig, checkOpts)
	if err != nil {
		return fmt.Errorf("verify blob: %w", err)
	}

	return nil
}

func createCosignCheckOpts(signatureOpts config.AgentSignature) (*cosign.CheckOpts, error) {
	rootCerts, err := fulcio.GetRoots()
	if err != nil {
		return nil, fmt.Errorf("fetch root certs: %w", err)
	}

	intermediateCerts, err := fulcio.GetIntermediates()
	if err != nil {
		return nil, fmt.Errorf("fetch intermediate certs: %w", err)
	}

	identities := make([]cosign.Identity, 0, len(signatureOpts.Identities))
	for _, ident := range signatureOpts.Identities {
		identities = append(identities, cosign.Identity{
			Issuer:        ident.Issuer,
			IssuerRegExp:  ident.IssuerRegExp,
			Subject:       ident.Subject,
			SubjectRegExp: ident.SubjectRegExp,
		})
	}

	return &cosign.CheckOpts{
		RootCerts:                    rootCerts,
		IntermediateCerts:            intermediateCerts,
		CertGithubWorkflowRepository: signatureOpts.CertGithubWorkflowRepository,
		Identities:                   identities,
	}, nil
}

func restoreBackup(backupPath, restorePath string) error {
	backupFile, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("open backup file: %w", err)
	}
	defer backupFile.Close()

	restoreFile, err := os.OpenFile(restorePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("open restore file: %w", err)
	}
	defer restoreFile.Close()

	if _, err := io.Copy(restoreFile, backupFile); err != nil {
		return fmt.Errorf("copy backup file to restore file: %w", err)
	}

	return nil
}
