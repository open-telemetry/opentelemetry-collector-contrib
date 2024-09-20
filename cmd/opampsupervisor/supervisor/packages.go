package supervisor

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
)

// agentPackageKey is the key used for the top-level package (the agent).
// According to the spec, an empty key may be used if there is only one top-level package.
// https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#packages
var (
	agentPackageKey = ""
)

type packageState struct {
	allPackagesHash []byte
}

type packageMetadata struct {
	PackageType int32
	Hash        []byte
	Version     string
}

// packageManager manages the persistent state of downloadable packages.
// It persists the packages to the file system.
// The general structure is like this:
//
//	  ${storage_dir}
//		 |- package-state.yaml
//		 |- last-reported-package-statuses.proto
//		 |- packages
//			|- {package_name}
//			   | metadata.yaml
//			   | content
type packageManager struct {
	currentState *packageState
	agentVersion string
	agentHash    []byte

	agentPath        string
	packageStatePath string
	storageDir       string
}

func newPackageManager(agentPath string, storageDir string) (*packageManager, error) {
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

	return &packageManager{
		currentState: state,
		agentHash:    agentHash,

		agentPath:        agentPath,
		storageDir:       storageDir,
		packageStatePath: packageStatePath,
	}, nil
}

func (p *packageManager) setAgentVersion(agentVersion string) {
	p.agentVersion = agentVersion
}

func (packageManager) AllPackagesHash() ([]byte, error) {
	return nil, fmt.Errorf("unimplemented")
}
func (packageManager) SetAllPackagesHash(hash []byte) error {
	return fmt.Errorf("unimplemented")
}
func (packageManager) Packages() ([]string, error) {
	return nil, fmt.Errorf("unimplemented")
}
func (packageManager) PackageState(packageName string) (state types.PackageState, err error) {
	return state, fmt.Errorf("unimplemented")
}
func (packageManager) SetPackageState(packageName string, state types.PackageState) error {
	return fmt.Errorf("unimplemented")
}
func (packageManager) CreatePackage(packageName string, typ protobufs.PackageType) error {
	return fmt.Errorf("unimplemented")
}
func (packageManager) FileContentHash(packageName string) ([]byte, error) {
	return nil, fmt.Errorf("unimplemented")
}
func (packageManager) UpdateContent(ctx context.Context, packageName string, data io.Reader, contentHash []byte) error {
	return fmt.Errorf("unimplemented")
}
func (packageManager) DeletePackage(packageName string) error {
	return fmt.Errorf("unimplemented")
}
func (p packageManager) LastReportedStatuses() (*protobufs.PackageStatuses, error) {
	return &protobufs.PackageStatuses{
		Packages: map[string]*protobufs.PackageStatus{
			agentPackageKey: {
				Name:                 agentPackageKey,
				AgentHasVersion:      p.agentVersion,
				AgentHasHash:         p.agentHash,
				ServerOfferedVersion: p.currentState.serverOfferedAgentVersion,
				ServerOfferedHash:    p.currentState.serverOfferedAgentHash,
				// TODO: set status failed if installing failed, for instance
				Status: protobufs.PackageStatusEnum_PackageStatusEnum_Installed,
			},
		},
		ServerProvidedAllPackagesHash: p.currentState.allPackagesHash,
	}, nil
}

func (packageManager) SetLastReportedStatuses(statuses *protobufs.PackageStatuses) error {
	return fmt.Errorf("unimplemented")
}

func (p *packageManager) setServerOfferedPackage(version string, hash, allHash []byte) error {
	p.currentState.serverOfferedAgentHash = hash
	p.currentState.serverOfferedAgentVersion = version
	p.currentState.allPackagesHash = allHash

	return savePackageState(p.packageStatePath, p.currentState)
}

func downloadPackageContent(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		return nil, fmt.Errorf("got non-200 status: %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return b, nil
}

func verifyPackageIntegrity(packageBytes, expectedHash []byte) error {
	actualHash := sha256.Sum256(packageBytes)
	if subtle.ConstantTimeCompare(actualHash[:], expectedHash) == 0 {
		return errors.New("invalid hash for package")
	}

	return nil
}

// sig is the decoded signature of
func verifyPackageSignature(packageBytes, b64Cert, b64Signature []byte) error {
	// TODO: allow specifying from config
	rootCerts, err := fulcio.GetRoots()
	if err != nil {
		return fmt.Errorf("fetch root certs: %w", err)
	}

	// TODO: allow specifying from config
	intermediateCerts, err := fulcio.GetIntermediates()
	if err != nil {
		return fmt.Errorf("fetch intermediate certs: %w", err)
	}

	co := &cosign.CheckOpts{
		RootCerts:         rootCerts,
		IntermediateCerts: intermediateCerts,
		// TODO: Make configurable
		CertGithubWorkflowRepository: "open-telemetry/opentelemetry-collector-releases",
		// TODO: Make allowed identities configurable
		Identities: []cosign.Identity{
			{
				SubjectRegExp: `^https://github.com/open-telemetry/opentelemetry-collector-releases/.github/workflows/base-release.yaml@refs/tags/[^/]*$`,
				Issuer:        "https://token.actions.githubusercontent.com",
			},
		},
	}

	decodedCert, err := base64.StdEncoding.AppendDecode(nil, b64Cert)
	if err != nil {
		return fmt.Errorf("b64 decode cert: %w", err)
	}

	// TODO: Should cert chain here be settable? Where should it come from?
	ociSig, err := static.NewSignature(packageBytes, string(b64Signature), static.WithCertChain(decodedCert, nil))
	if err != nil {
		return fmt.Errorf("create signature: %w", err)
	}

	_, err = cosign.VerifyBlobSignature(context.TODO(), ociSig, co)
	if err != nil {
		return fmt.Errorf("verify blob: %w", err)
	}

	return nil
}

func loadPackageState(path string) (*packageState, error) {
	return nil, fmt.Errorf("unimplemented")
}

func savePackageState(path string, ps *packageState) error {
	return fmt.Errorf("unimplemented")
}
