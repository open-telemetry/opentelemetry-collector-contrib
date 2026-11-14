// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/archive"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/verifier"
)

const (
	// agentPackageKey is the key of the top-level package (the agent). The spec
	// allows an empty key when there is a single top-level package.
	// https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#packages
	agentPackageKey = ""

	lastPackageStatusFileName = "last-reported-package-statuses.binpb"

	// stagedAgentFileName is the file in the storage directory that a verified
	// agent binary is extracted to before the supervisor installs it.
	stagedAgentFileName = "collector.tmp"
)

// agentManager installs a staged agent binary. The supervisor implements it: it
// stops the running agent, moves the staged binary into place, starts it, and
// restores the previous binary if the new one fails to start.
type agentManager interface {
	// installAgentBinary returns nil once the binary at stagedPath is installed
	// and running, or an error once the previous binary has been restored.
	installAgentBinary(ctx context.Context, stagedPath string) error
}

// packageManager manages the persistent state of downloadable packages.
// Currently only allows for a single top-level package containing the agent.
type packageManager struct {
	logger *zap.Logger

	// persistentState stores AllPackagesHash across supervisor restarts.
	persistentState *persistentState
	// topLevelHash and topLevelVersion are the agent package's hash and version
	// as last set by the OpAMP client through SetPackageState. Until then they
	// describe the on-disk agent binary.
	topLevelHash    []byte
	topLevelVersion string

	storageDir  string
	agentBinary string
	verifierCfg config.Verifier
	// newVerifier builds the package verifier when a package arrives rather than
	// at construction: the cosign verifier fetches the Sigstore trusted root over
	// the network, which must not happen at supervisor startup. Overridden in
	// tests.
	newVerifier func(config.Verifier) (verifier.Verifier, error)
	// maxPackageBytes bounds how much of a downloaded package is read into
	// memory. Lowered in tests.
	maxPackageBytes int64
	am              agentManager
}

var _ types.PackagesStateProvider = &packageManager{}

func newPackageManager(
	logger *zap.Logger,
	agentPath, storageDir, agentVersion string,
	persistentState *persistentState,
	packageOpts config.AgentPackage,
	am agentManager,
) (*packageManager, error) {
	agentHash, err := fileSHA256(agentPath)
	if err != nil {
		return nil, fmt.Errorf("hash agent: %w", err)
	}

	return &packageManager{
		logger:          logger,
		persistentState: persistentState,
		topLevelHash:    agentHash,
		topLevelVersion: agentVersion,
		storageDir:      storageDir,
		agentBinary:     packageOpts.AgentBinary,
		verifierCfg:     packageOpts.Verifier,
		newVerifier:     verifier.NewVerifier,
		maxPackageBytes: archive.MaxAgentBytes,
		am:              am,
	}, nil
}

func (p *packageManager) AllPackagesHash() ([]byte, error) {
	return p.persistentState.GetAllPackagesHash(), nil
}

func (p *packageManager) SetAllPackagesHash(hash []byte) error {
	return p.persistentState.SetAllPackagesHash(hash)
}

func (*packageManager) Packages() ([]string, error) {
	return []string{agentPackageKey}, nil
}

func (p *packageManager) PackageState(packageName string) (types.PackageState, error) {
	if packageName != agentPackageKey {
		return types.PackageState{Exists: false}, nil
	}

	return types.PackageState{
		Exists:  true,
		Type:    protobufs.PackageType_PackageType_TopLevel,
		Hash:    p.topLevelHash,
		Version: p.topLevelVersion,
	}, nil
}

func (p *packageManager) SetPackageState(packageName string, state types.PackageState) error {
	if packageName != agentPackageKey {
		return fmt.Errorf("package %q does not exist", packageName)
	}
	if !state.Exists {
		return errors.New("agent package must be marked as existing")
	}
	if state.Type != protobufs.PackageType_PackageType_TopLevel {
		return errors.New("agent package must be marked as top level")
	}

	p.topLevelHash = state.Hash
	p.topLevelVersion = state.Version
	return nil
}

func (*packageManager) CreatePackage(packageName string, _ protobufs.PackageType) error {
	if packageName != agentPackageKey {
		return errors.New("only agent package is supported")
	}
	return errors.New("agent package already exists")
}

func (p *packageManager) FileContentHash(packageName string) ([]byte, error) {
	if packageName != agentPackageKey {
		return nil, nil
	}
	return p.topLevelHash, nil
}

// UpdateContent installs a new agent package:
//  1. reads the package, bounded by maxPackageBytes
//  2. verifies its sha256 matches contentHash
//  3. verifies its signature with the configured verifier
//  4. extracts the agent binary to a staging file in the storage directory,
//     detecting the archive format from downloadURL
//  5. hands the staged binary to the supervisor, which swaps it in, starts it,
//     and restores the previous binary if the new one fails to start
//
// It returns nil only once the new agent binary is running, so the OpAMP client
// reports Installed and InstallFailed accurately.
func (p *packageManager) UpdateContent(ctx context.Context, packageName, downloadURL string, data io.Reader, contentHash, signature []byte) error {
	if packageName != agentPackageKey {
		return errors.New("package does not exist")
	}

	p.logger.Info("Begin agent binary update.", zap.String("download_url", downloadURL))

	pkg, err := io.ReadAll(io.LimitReader(data, p.maxPackageBytes+1))
	if err != nil {
		return fmt.Errorf("read package: %w", err)
	}
	if int64(len(pkg)) > p.maxPackageBytes {
		return fmt.Errorf("package exceeds maximum size of %d bytes", p.maxPackageBytes)
	}

	if err = verifyPackageHash(pkg, contentHash); err != nil {
		return fmt.Errorf("could not verify package integrity: %w", err)
	}

	v, err := p.newVerifier(p.verifierCfg)
	if err != nil {
		return fmt.Errorf("create package verifier: %w", err)
	}
	p.logger.Debug("Verifying package signature", zap.String("verifier_type", v.Type()))
	if err = v.Verify(pkg, signature); err != nil {
		return fmt.Errorf("could not verify package signature: %w", err)
	}

	format := archiveFormat(downloadURL)
	extractor, err := archive.NewExtractor(format)
	if err != nil {
		return fmt.Errorf("create extractor: %w", err)
	}
	stagedPath := filepath.Join(p.storageDir, stagedAgentFileName)
	p.logger.Debug("Extracting agent binary", zap.String("format", string(format)), zap.String("path", stagedPath))
	if err = extractor.Extract(ctx, pkg, p.agentBinary, stagedPath); err != nil {
		return fmt.Errorf("extract agent binary: %w", err)
	}

	if err = p.am.installAgentBinary(ctx, stagedPath); err != nil {
		if rmErr := os.Remove(stagedPath); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			p.logger.Error("Failed to remove staged agent binary.", zap.Error(rmErr))
		}
		return fmt.Errorf("install agent binary: %w", err)
	}

	p.logger.Info("Agent binary update completed successfully.")
	return nil
}

func (*packageManager) DeletePackage(packageName string) error {
	if packageName != agentPackageKey {
		// Only the agent package is accepted, so any other package already does not exist.
		return nil
	}
	return errors.New("cannot delete top-level package")
}

func (p *packageManager) LastReportedStatuses() (*protobufs.PackageStatuses, error) {
	by, err := os.ReadFile(p.lastPackageStatusPath())
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("read last package statuses: %w", err)
	}

	var statuses protobufs.PackageStatuses
	if err := proto.Unmarshal(by, &statuses); err != nil {
		return nil, fmt.Errorf("unmarshal last package statuses: %w", err)
	}
	return &statuses, nil
}

func (p *packageManager) SetLastReportedStatuses(statuses *protobufs.PackageStatuses) error {
	by, err := proto.Marshal(statuses)
	if err != nil {
		return fmt.Errorf("marshal package statuses: %w", err)
	}
	if err := os.WriteFile(p.lastPackageStatusPath(), by, 0o600); err != nil {
		return fmt.Errorf("write package statuses: %w", err)
	}
	return nil
}

func (p *packageManager) lastPackageStatusPath() string {
	return filepath.Join(p.storageDir, lastPackageStatusFileName)
}

// archiveFormat detects the package format from the download URL's path. The
// OpAMP client passes only the URL; there is no Content-Type at this layer.
func archiveFormat(downloadURL string) archive.Format {
	path := downloadURL
	if u, err := url.Parse(downloadURL); err == nil {
		path = u.Path
	}
	if strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz") {
		return archive.FormatTarGzip
	}
	return archive.FormatNone
}

func verifyPackageHash(pkg, expectedHash []byte) error {
	actualHash := sha256.Sum256(pkg)
	if !bytes.Equal(actualHash[:], expectedHash) {
		return errors.New("invalid hash for package")
	}
	return nil
}

func fileSHA256(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}
