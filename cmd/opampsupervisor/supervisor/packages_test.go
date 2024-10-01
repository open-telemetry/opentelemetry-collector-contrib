// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

const testAgentFileContents = "agent file"

var testAgentFileHash = []byte{
	0x67, 0x9e, 0x2a, 0x15, 0xae, 0x3d, 0xc, 0xa7,
	0xa0, 0xd7, 0xf, 0xc, 0x28, 0x7a, 0x22, 0x8f,
	0xd, 0xe5, 0x6a, 0xb6, 0xa7, 0x40, 0xe6, 0x52,
	0x73, 0x7c, 0x75, 0x1d, 0xec, 0x56, 0xfc, 0xd7,
}

func TestNewPackageManager(t *testing.T) {
	t.Run("Creates new package manager", func(t *testing.T) {
		tmpDir := t.TempDir()
		agentFile := filepath.Join(tmpDir, "agent")
		storageDir := filepath.Join(tmpDir, "storage")
		defaultSigOpts := config.DefaultSupervisor().Agent.Signature

		require.NoError(t, os.MkdirAll(storageDir, 0700))
		require.NoError(t, os.WriteFile(agentFile, []byte(testAgentFileContents), 0600))

		pm, err := newPackageManager(agentFile, storageDir, "v0.110.0", defaultSigOpts, nil)
		require.NoError(t, err)

		assert.Equal(t, "v0.110.0", pm.topLevelVersion)
		assert.Equal(t, testAgentFileHash, pm.topLevelHash)
		assert.Equal(t, &packageState{}, pm.packageState)
	})

	t.Run("Package states is loaded from disk", func(t *testing.T) {
		tmpDir := t.TempDir()
		agentFile := filepath.Join(tmpDir, "agent")
		storageDir := filepath.Join(tmpDir, "storage")
		packageStateFile := filepath.Join(storageDir, packagesStateFileName)
		defaultSigOpts := config.DefaultSupervisor().Agent.Signature

		require.NoError(t, os.MkdirAll(storageDir, 0700))
		require.NoError(t, os.WriteFile(agentFile, []byte(testAgentFileContents), 0600))
		require.NoError(t, os.WriteFile(packageStateFile, []byte(`
all_packages_hash: "679e2a15ae3d0ca7a0d70f0c287a228f0de56ab6a740e652737c751dec56fcd7"
`), 0600))

		pm, err := newPackageManager(agentFile, storageDir, "v0.110.0", defaultSigOpts, nil)
		require.NoError(t, err)

		assert.Equal(t, "v0.110.0", pm.topLevelVersion)
		assert.Equal(t, testAgentFileHash, pm.topLevelHash)
		assert.Equal(t, &packageState{
			AllPackagesHash: testAgentFileHash,
		}, pm.packageState)
	})

	t.Run("Malformed package state on disk", func(t *testing.T) {
		tmpDir := t.TempDir()
		agentFile := filepath.Join(tmpDir, "agent")
		storageDir := filepath.Join(tmpDir, "storage")
		packageStateFile := filepath.Join(storageDir, packagesStateFileName)
		defaultSigOpts := config.DefaultSupervisor().Agent.Signature

		require.NoError(t, os.MkdirAll(storageDir, 0700))
		require.NoError(t, os.WriteFile(agentFile, []byte(testAgentFileContents), 0600))
		require.NoError(t, os.WriteFile(packageStateFile, []byte(`
all_packages_hash: "malformed""
`), 0600))

		_, err := newPackageManager(agentFile, storageDir, "v0.110.0", defaultSigOpts, nil)
		require.ErrorContains(t, err, "load package state:")
	})

	t.Run("Agent does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		agentFile := filepath.Join(tmpDir, "agent")
		storageDir := filepath.Join(tmpDir, "storage")
		defaultSigOpts := config.DefaultSupervisor().Agent.Signature

		require.NoError(t, os.MkdirAll(storageDir, 0700))

		_, err := newPackageManager(agentFile, storageDir, "v0.110.0", defaultSigOpts, nil)
		require.ErrorContains(t, err, "open agent:")
	})
}

func TestPackageManager_AllPackagesHash(t *testing.T) {
	tmpDir := t.TempDir()
	pm1 := initPackageManager(t, tmpDir)
	allPackagesHash := []byte{0x0, 0x1}

	err := pm1.SetAllPackagesHash(allPackagesHash)
	require.NoError(t, err)

	// Assert set hash is returned by AllPackagesHash
	by, err := pm1.AllPackagesHash()
	require.NoError(t, err)
	require.Equal(t, allPackagesHash, by)

	// Assert that all packages hash is persisted, even between different
	// instances of packageManager
	pm2 := initPackageManager(t, tmpDir)

	by, err = pm2.AllPackagesHash()
	require.NoError(t, err)
	require.Equal(t, allPackagesHash, by)

}

func TestPackageManager_Packages(t *testing.T) {
	pm := initPackageManager(t, t.TempDir())
	pkgs, err := pm.Packages()
	require.NoError(t, err)
	require.Equal(t, []string{agentPackageKey}, pkgs)
}

func TestPackageManager_PackageState(t *testing.T) {
	t.Run("Set non-agent package", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		err := pm.SetPackageState("random-package", types.PackageState{})
		require.Equal(t, `package "random-package" does not exist`, err.Error())
	})

	t.Run("Set agent package to non-existent", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		err := pm.SetPackageState(agentPackageKey, types.PackageState{
			Exists:  false,
			Type:    protobufs.PackageType_PackageType_TopLevel,
			Hash:    []byte{0x01, 0x02},
			Version: "v0.111.0",
		})
		require.Equal(t, `agent package must be marked as existing`, err.Error())
	})

	t.Run("Set agent package to non-top-level", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		err := pm.SetPackageState(agentPackageKey, types.PackageState{
			Exists:  true,
			Type:    protobufs.PackageType_PackageType_Addon,
			Hash:    []byte{0x01, 0x02},
			Version: "v0.111.0",
		})
		require.Equal(t, `agent package must be marked as top level`, err.Error())
	})

	t.Run("Set agent package hash/version", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		ps := types.PackageState{
			Exists:  true,
			Type:    protobufs.PackageType_PackageType_TopLevel,
			Hash:    []byte{0x01, 0x02},
			Version: "v0.111.0",
		}

		err := pm.SetPackageState(agentPackageKey, ps)
		require.NoError(t, err)

		s, err := pm.PackageState(agentPackageKey)
		require.NoError(t, err)
		require.Equal(t, ps, s)
	})

	t.Run("Get non-existent package state", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		ps, err := pm.PackageState("random-package")
		require.NoError(t, err)
		require.False(t, ps.Exists)
	})
}

func TestPackageManager_CreatePackage(t *testing.T) {
	t.Run("Try create non-agent package", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		err := pm.CreatePackage("random-package", protobufs.PackageType_PackageType_Addon)
		require.Equal(t, "only agent package is supported", err.Error())
	})
	t.Run("Try create agent package", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		err := pm.CreatePackage(agentPackageKey, protobufs.PackageType_PackageType_Addon)
		require.Equal(t, "agent package already exists", err.Error())
	})
}

func TestPackageManager_FileContentHash(t *testing.T) {
	t.Run("non-agent package", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		hash, err := pm.FileContentHash("random-package")
		require.NoError(t, err)
		require.Nil(t, hash)
	})

	t.Run("agent package", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		hash, err := pm.FileContentHash(agentPackageKey)
		require.NoError(t, err)
		require.Equal(t, testAgentFileHash, hash)
	})
}

func TestPackageManager_DeletePackage(t *testing.T) {
	t.Run("non-agent package", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		err := pm.DeletePackage("random-package")
		require.NoError(t, err)
	})

	t.Run("agent package", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		err := pm.DeletePackage(agentPackageKey)
		require.Equal(t, "cannot delete top-level package", err.Error())
	})
}

func TestPackageManager_LastReportedStatuses(t *testing.T) {
	tmpDir := t.TempDir()
	pm1 := initPackageManager(t, tmpDir)
	statuses := &protobufs.PackageStatuses{
		Packages: map[string]*protobufs.PackageStatus{
			agentPackageKey: {
				Name:                 agentPackageKey,
				AgentHasVersion:      "v0.101.0",
				AgentHasHash:         []byte{0x01, 0x02},
				ServerOfferedVersion: "v0.102.0",
				ServerOfferedHash:    []byte{0x03, 0x04},
			},
		},
		ServerProvidedAllPackagesHash: []byte{0x01, 0x02},
	}

	// clone statuses for test since marshaling mutates internal state
	clonedStatuses := proto.Clone(statuses).(*protobufs.PackageStatuses)

	err := pm1.SetLastReportedStatuses(clonedStatuses)
	require.NoError(t, err)

	// Assert set statuses is returned by LastReportedStatuses
	lrs, err := pm1.LastReportedStatuses()
	require.NoError(t, err)
	require.Equal(t, statuses, lrs)

	// Assert that statuses are persisted, even between different
	// instances of packageManager
	pm2 := initPackageManager(t, tmpDir)

	lrs, err = pm2.LastReportedStatuses()
	require.NoError(t, err)
	require.Equal(t, statuses, lrs)
}

func initPackageManager(t *testing.T, tmpDir string) *packageManager {
	agentFile := filepath.Join(tmpDir, "agent")
	storageDir := filepath.Join(tmpDir, "storage")
	defaultSigOpts := config.DefaultSupervisor().Agent.Signature

	require.NoError(t, os.MkdirAll(storageDir, 0700))
	require.NoError(t, os.WriteFile(agentFile, []byte(testAgentFileContents), 0600))

	pm, err := newPackageManager(agentFile, storageDir, "v0.110.0", defaultSigOpts, nil)
	require.NoError(t, err)

	return pm
}
