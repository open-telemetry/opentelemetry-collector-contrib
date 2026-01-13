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
	"go.uber.org/zap"
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
	logger *zap.Logger

	// persistentState is used to track the AllPackagesHash, currently this should evaluate to just the collector package hash
	persistentState *persistentState
	// topLevelHash is the collector package hash from the Hash object in the PackageAvailable OpAmp message handled by the OpAmp.Client
	topLevelHash []byte
	// topLevelVersion is the collector package version from the Version object in the PackageAvailable OpAmp message handled by the OpAmp.Client
	topLevelVersion string

	storageDir   string
	agentExePath string
	installFunc  installFunc
	agentBinary  string
	verifier     verifier.Verifier
	am           agentManager
}

var _ types.PackagesStateProvider = &packageManager{}

type agentManager interface {
	// updateStopAgentProcess stops the agent process.
	// It returns a channel that can be closed to signal the agent should be restarted,
	// a channel that can be used to signal to the runAgentProcess goroutine to start the updated collector binary,
	// and a channel that can be used to communicate the outcome of starting the updated binary.
	updateStopAgentProcess(ctx context.Context) (chan struct{}, chan string, chan error, error)
}

func newPackageManager(
	logger *zap.Logger,
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
		logger:          logger,
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
// 1. Read data from the data stream.
// 2. Verify the package data matches the expected content hash.
// 3. Verify the package signature using the configured Verifier.
// 4. Verify and install the agent binary from the archive.
// 5. Stop the existing agent process.
// 6. Backup the existing agent binary.
// 7. Write the new agent binary to the configured agent binary path.
// 8. Signal to start the updated agent binary and wait for the outcome.
// 9. Delete the backup agent binary after a successful update.
//
// If successful, the updated agent binary has been started when this function returns.
// If the update fails, an attempt to restore and start the backup agent binary will be made.
func (p *packageManager) UpdateContent(ctx context.Context, packageName string, data io.Reader, contentHash, signature []byte) error {
	// Only the agent package is supported.
	if packageName != agentPackageKey {
		return errors.New("package does not exist")
	}

	p.logger.Info("Begin agent binary update.")

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
	p.logger.Debug("Verifying package signature", zap.String("verifier type", p.verifier.Type()))
	if err = p.verifier.Verify(by, signature); err != nil {
		return fmt.Errorf("could not verify package signature: %w", err)
	}

	// 4. Verify and install the agent binary from the archive.
	p.logger.Debug("Verifying and installing agent binary from archive")
	tmpFilePath := filepath.Join(p.storageDir, "collector.tmp")
	if err = p.installFunc(ctx, by, p.agentBinary, tmpFilePath); err != nil {
		return fmt.Errorf("install func: %w", err)
	}

	// 5. Stop the existing agent process.
	restartProcess, startUpdatedBinary, startUpdatedBinaryOutcome, err := p.am.updateStopAgentProcess(ctx)
	if err != nil {
		return fmt.Errorf("stop agent process: %w", err)
	}

	p.logger.Debug("Current agent process stopped. Backing up existing agent binary and writing new agent binary.")

	// 6. Backup the existing agent binary.
	agentBackupPath := filepath.Join(p.storageDir, "collector.bak")
	if err = moveFile(p.agentExePath, agentBackupPath); err != nil {
		p.logger.Error("Failed to backup existing agent binary. Signaling to restart existing process.", zap.Error(err))
		close(restartProcess)
		return fmt.Errorf("move existing agent binary to backup path: %w", err)
	}

	// 7. Write the new agent binary to the configured agent binary path.
	if err = moveFile(tmpFilePath, p.agentExePath); err != nil {
		writeErr := fmt.Errorf("write new agent binary to agent executable path: %w", err)
		// restore the backup agent binary
		if restoreErr := moveFile(agentBackupPath, p.agentExePath); restoreErr != nil {
			joinedErr := errors.Join(writeErr, fmt.Errorf("restore agent backup: %w", restoreErr))
			// failed to restore backup, don't attempt to start the backup
			p.logger.Error("Failed to write new agent binary to agent executable path. Failed to restore backup. Not signaling to restart existing process.", zap.Error(joinedErr))
			return joinedErr
		}
		// successfully restored backup, signal to start the backup
		p.logger.Error("Failed to write new agent binary to agent executable path. Signaling to restart existing process.", zap.Error(writeErr))
		close(restartProcess)
		return writeErr
	}

	// 8. Signal to start the updated agent binary and wait for the outcome.
	p.logger.Debug("New agent binary written to agent executable path. Signaling to start updated agent binary.")
	startUpdatedBinary <- agentBackupPath
	select {
	case err := <-startUpdatedBinaryOutcome:
		// failed to start the updated agent binary
		if err != nil {
			return fmt.Errorf("start updated agent binary: %w", err)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	// 9. Delete the backup agent binary after a successful update.
	if err = os.Remove(agentBackupPath); err != nil {
		// this error should not prevent reporting a successful update
		p.logger.Error("Failed to delete backup agent binary after successful update.", zap.Error(err))
	}

	p.logger.Info("Agent binary update completed successfully.")

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

// moveFile will move the file at srcPath to dstPath using os.Rename
// verifies the dstPath is cleared up, calling os.Remove, so a clean rename can occur
func moveFile(srcPath, dstPath string) error {
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
