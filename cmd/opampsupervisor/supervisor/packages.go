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
	"os"
	"path/filepath"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/verifier"
)

var (
	// agentPackageKey is the key used for the top-level package (the agent).
	// According to the spec, an empty key may be used if there is only one top-level package.
	// https://github.com/open-telemetry/opamp-spec/blob/main/specification.md#packages
	agentPackageKey = ""

	lastPackageStatusFileName = "last-reported-package-statuses.binpb"
)

// packageManager manages the persistent state of downloadable packages.
// It persists the packages to the file system.
// Currently, it only allows for a single top-level package containing the agent
// to be received.
type packageManager struct {
	// persistentState is used to track the AllPackagesHash, currently this should evaluate to just the collector package hash
	persistentState *persistentState
	// topLevelHash is the collector package hash from the Hash object in the PackageAvailable OpAmp message handled by the OpAmp.Client
	topLevelHash []byte
	// topLevelVersion is the collector package version from the Version object in the PackageAvailable OpAmp message handled by the OpAmp.Client
	topLevelVersion string

	storageDir   string
	agentExePath string
	installFunc  InstallFunc
	agentBinary  string
	verifier     verifier.Verifier
	am           agentManager
}

var _ types.PackagesStateProvider = &packageManager{}

type agentManager interface {
	// stopAgentProcess stops the agent process.
	// It returns a channel that can be closed to signal the agent should be started again.
	stopAgentProcess(ctx context.Context) (chan struct{}, error)
}

func newPackageManager(
	agentPath,
	storageDir,
	agentVersion string,
	persistentState *persistentState,
	packageOpts config.AgentPackage,
	am agentManager,
) (*packageManager, error) {
	// Calculate actual hash of the on-disk agent
	f, err := os.Open(agentPath)
	if err != nil {
		return nil, fmt.Errorf("open agent: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("compute agent hash: %w", err)
	}
	agentHash := h.Sum(nil)

	// Create archive installer
	installer, err := NewInstallFunc(packageOpts.Archive)
	if err != nil {
		return nil, fmt.Errorf("create archive installer: %w", err)
	}

	// Create package verifier
	verifier, err := verifier.NewVerifier(packageOpts.Verifier)
	if err != nil {
		return nil, fmt.Errorf("create package verifier: %w", err)
	}

	return &packageManager{
		persistentState: persistentState,
		topLevelHash:    agentHash,
		topLevelVersion: agentVersion,
		storageDir:      storageDir,
		agentExePath:    agentPath,
		installFunc:     installer,
		agentBinary:     packageOpts.AgentBinary,
		verifier:        verifier,
		am:              am,
	}, nil
}

func (p *packageManager) AllPackagesHash() ([]byte, error) {
	p.persistentState.mux.Lock()
	defer p.persistentState.mux.Unlock()
	return p.persistentState.AllPackagesHash, nil
}

func (p *packageManager) SetAllPackagesHash(hash []byte) error {
	p.persistentState.mux.Lock()
	defer p.persistentState.mux.Unlock()
	return p.persistentState.SetAllPackagesHash(hash)
}

func (packageManager) Packages() ([]string, error) {
	return []string{agentPackageKey}, nil
}

func (p *packageManager) PackageState(packageName string) (state types.PackageState, err error) {
	if packageName == agentPackageKey {
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
		return errors.New("agent package must be marked as existing")
	}

	if state.Type != protobufs.PackageType_PackageType_TopLevel {
		return errors.New("agent package must be marked as top level")
	}

	p.topLevelHash = state.Hash
	p.topLevelVersion = state.Version

	return nil
}

func (packageManager) CreatePackage(packageName string, _ protobufs.PackageType) error {
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

// UpdateContent updates the content of the agent package.
// The order of operations is as follows:
// 1. Download data from the data stream.
// 2. Verify the package data matches the expected content hash.
// 3. Verify the package signature using the configured Verifier.
// 4. Verify the tarball contains the collector binary.
// 5. Extract data from tarball to a temporary file.
// 6. Stop the agent process.
// 7. Backup the existing agent file.
// 8. Overwrite the existing agent file with the new agent.
// 9. Delete the backup after a successful update.
// Agent process is restarted when this function returns.
func (p *packageManager) UpdateContent(ctx context.Context, packageName string, data io.Reader, contentHash, signature []byte) error {
	// Only the agent package is supported.
	if packageName != agentPackageKey {
		return errors.New("package does not exist")
	}

	// 1. Read data from the data stream.
	by, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("read package bytes: %w", err)
	}

	// 2. Verify the package data matches the expected content hash.
	if err = verifyPackageHash(by, contentHash); err != nil {
		return fmt.Errorf("could not verify package integrity: %w", err)
	}

	// 3. Verify the package signature using the configured Verifier.
	if err = p.verifier.Verify(by, signature); err != nil {
		return fmt.Errorf("could not verify package signature: %w", err)
	}

	// 4, 5: Verify and install the agent binary to a temporary file.
	tmpFilePath := filepath.Join(p.storageDir, "collector.tmp")
	if err = p.installFunc(ctx, by, p.agentBinary, tmpFilePath); err != nil {
		return fmt.Errorf("install func: %w", err)
	}

	// 6. Stop the agent process.
	startAgent, err := p.am.stopAgentProcess(ctx)
	if err != nil {
		return fmt.Errorf("stop agent process: %w", err)
	}

	// We always want to start the agent process again, even if we fail to write the agent file
	defer close(startAgent)

	// 7. Backup the existing agent file.
	agentBackupPath := filepath.Join(p.storageDir, "collector.bak")
	if err = renameFile(p.agentExePath, agentBackupPath); err != nil {
		return fmt.Errorf("rename collector exe path to backup path: %w", err)
	}

	// 8. Overwrite the existing agent file with the new agent.
	if err = renameFile(tmpFilePath, p.agentExePath); err != nil {
		if restoreErr := renameFile(agentBackupPath, p.agentExePath); restoreErr != nil {
			return errors.Join(fmt.Errorf("rename tmp file to agent executable path: %w", err), fmt.Errorf("restore agent backup: %w", restoreErr))
		}
		return fmt.Errorf("successfully restored backup, but failed to rename tmp file to agent executable path: %w", err)
	}

	// 9. Delete the backup after a successful update.
	if err = os.Remove(agentBackupPath); err != nil {
		return fmt.Errorf("delete agent backup: %w", err)
	}

	return nil
}

func (packageManager) DeletePackage(packageName string) error {
	if packageName != agentPackageKey {
		// We only take the agent package, so the package already doesn't exist.
		return nil
	}

	// We will never delete the agent package.
	return errors.New("cannot delete top-level package")
}

func (p *packageManager) LastReportedStatuses() (*protobufs.PackageStatuses, error) {
	lastStatusBytes, err := os.ReadFile(p.lastPackageStatusPath())
	switch {
	case errors.Is(err, os.ErrNotExist):
		// No package statuses exists
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("read last package statuses: %w", err)
	}

	var ret protobufs.PackageStatuses
	err = proto.Unmarshal(lastStatusBytes, &ret)
	if err != nil {
		return nil, fmt.Errorf("unmarshal last package statuses: %w", err)
	}

	return &ret, nil
}

func (p *packageManager) SetLastReportedStatuses(statuses *protobufs.PackageStatuses) error {
	lastStatusBytes, err := proto.Marshal(statuses)
	if err != nil {
		return fmt.Errorf("marshal statuses: %w", err)
	}

	err = os.WriteFile(p.lastPackageStatusPath(), lastStatusBytes, 0o600)
	if err != nil {
		return fmt.Errorf("write package statues: %w", err)
	}

	return nil
}

func (p *packageManager) lastPackageStatusPath() string {
	return filepath.Join(p.storageDir, lastPackageStatusFileName)
}

func verifyPackageHash(packageBytes, expectedHash []byte) error {
	actualHash := sha256.Sum256(packageBytes)
	if !bytes.Equal(actualHash[:], expectedHash) {
		return errors.New("invalid hash for package")
	}

	return nil
}

// renameFile will rename the file at srcPath to dstPath
// verifies the dstPath is cleared up, calling os.Remove, so a clean rename can occur
func renameFile(srcPath, dstPath string) error {
	// verify dstPath is cleared up
	if _, err := os.Stat(dstPath); err == nil {
		// delete existing file at dstPath
		if err = os.Remove(dstPath); err != nil {
			return fmt.Errorf("remove existing file at destination path: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check destination path: %w", err)
	}
	if err := os.Rename(srcPath, dstPath); err != nil {
		return fmt.Errorf("rename source to destination: %w", err)
	}
	return nil
}
