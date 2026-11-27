// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/archive"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/verifier"
)

const testAgentFileContents = "agent file"

var testAgentFileHash = []byte{
	0x67, 0x9e, 0x2a, 0x15, 0xae, 0x3d, 0xc, 0xa7,
	0xa0, 0xd7, 0xf, 0xc, 0x28, 0x7a, 0x22, 0x8f,
	0xd, 0xe5, 0x6a, 0xb6, 0xa7, 0x40, 0xe6, 0x52,
	0x73, 0x7c, 0x75, 0x1d, 0xec, 0x56, 0xfc, 0xd7,
}

// agentManagerFunc adapts a func to the agentManager interface for tests.
type agentManagerFunc func(ctx context.Context, stagedPath string) error

func (f agentManagerFunc) installAgentBinary(ctx context.Context, stagedPath string) error {
	return f(ctx, stagedPath)
}

// failingVerifier rejects every signature.
type failingVerifier struct{}

func (failingVerifier) Verify(_, _ []byte) error { return errors.New("bad signature") }
func (failingVerifier) Type() string             { return "failing" }

func TestNewPackageManager(t *testing.T) {
	t.Run("Creates new package manager", func(t *testing.T) {
		tmpDir := t.TempDir()
		agentFile := filepath.Join(tmpDir, "agent")
		storageDir := filepath.Join(tmpDir, "storage")

		require.NoError(t, os.MkdirAll(storageDir, 0o700))
		require.NoError(t, os.WriteFile(agentFile, []byte(testAgentFileContents), 0o600))

		pm, err := newPackageManager(zap.NewNop(), agentFile, storageDir, "v0.110.0", &persistentState{}, config.DefaultSupervisor().Agent.Package, nil)
		require.NoError(t, err)

		assert.Equal(t, "v0.110.0", pm.topLevelVersion)
		assert.Equal(t, testAgentFileHash, pm.topLevelHash)
	})

	t.Run("Agent does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		agentFile := filepath.Join(tmpDir, "agent")
		storageDir := filepath.Join(tmpDir, "storage")

		require.NoError(t, os.MkdirAll(storageDir, 0o700))

		_, err := newPackageManager(zap.NewNop(), agentFile, storageDir, "v0.110.0", &persistentState{}, config.DefaultSupervisor().Agent.Package, nil)
		require.ErrorContains(t, err, "hash agent:")
	})
}

func TestPackageManager_AllPackagesHash(t *testing.T) {
	tmpDir := t.TempDir()
	pm1 := initPackageManager(t, tmpDir)
	allPackagesHash := []byte{0x0, 0x1}

	require.NoError(t, pm1.SetAllPackagesHash(allPackagesHash))

	by, err := pm1.AllPackagesHash()
	require.NoError(t, err)
	require.Equal(t, allPackagesHash, by)

	// The hash is persisted across package manager instances.
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
		require.EqualError(t, err, `package "random-package" does not exist`)
	})

	t.Run("Set agent package to non-existent", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		err := pm.SetPackageState(agentPackageKey, types.PackageState{
			Exists:  false,
			Type:    protobufs.PackageType_PackageType_TopLevel,
			Hash:    []byte{0x01, 0x02},
			Version: "v0.111.0",
		})
		require.EqualError(t, err, "agent package must be marked as existing")
	})

	t.Run("Set agent package to non-top-level", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		err := pm.SetPackageState(agentPackageKey, types.PackageState{
			Exists:  true,
			Type:    protobufs.PackageType_PackageType_Addon,
			Hash:    []byte{0x01, 0x02},
			Version: "v0.111.0",
		})
		require.EqualError(t, err, "agent package must be marked as top level")
	})

	t.Run("Set agent package hash/version", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		ps := types.PackageState{
			Exists:  true,
			Type:    protobufs.PackageType_PackageType_TopLevel,
			Hash:    []byte{0x01, 0x02},
			Version: "v0.111.0",
		}

		require.NoError(t, pm.SetPackageState(agentPackageKey, ps))

		s, err := pm.PackageState(agentPackageKey)
		require.NoError(t, err)
		require.Equal(t, ps, s)

		hash, err := pm.FileContentHash(agentPackageKey)
		require.NoError(t, err)
		require.Equal(t, ps.Hash, hash)
	})

	t.Run("Get non-existent package state", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		ps, err := pm.PackageState("random-package")
		require.NoError(t, err)
		require.False(t, ps.Exists)
	})
}

func TestPackageManager_CreatePackage(t *testing.T) {
	pm := initPackageManager(t, t.TempDir())
	require.EqualError(t, pm.CreatePackage("random-package", protobufs.PackageType_PackageType_Addon), "only agent package is supported")
	require.EqualError(t, pm.CreatePackage(agentPackageKey, protobufs.PackageType_PackageType_TopLevel), "agent package already exists")
}

func TestPackageManager_FileContentHash(t *testing.T) {
	pm := initPackageManager(t, t.TempDir())

	hash, err := pm.FileContentHash("random-package")
	require.NoError(t, err)
	require.Nil(t, hash)

	hash, err = pm.FileContentHash(agentPackageKey)
	require.NoError(t, err)
	require.Equal(t, testAgentFileHash, hash)
}

func TestPackageManager_DeletePackage(t *testing.T) {
	pm := initPackageManager(t, t.TempDir())
	require.NoError(t, pm.DeletePackage("random-package"))
	require.EqualError(t, pm.DeletePackage(agentPackageKey), "cannot delete top-level package")
}

func TestPackageManager_LastReportedStatuses(t *testing.T) {
	tmpDir := t.TempDir()
	pm1 := initPackageManager(t, tmpDir)

	// Nothing reported yet.
	lrs, err := pm1.LastReportedStatuses()
	require.NoError(t, err)
	require.Nil(t, lrs)

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

	// Marshaling mutates internal proto state, so hand the manager a clone.
	require.NoError(t, pm1.SetLastReportedStatuses(proto.Clone(statuses).(*protobufs.PackageStatuses)))

	lrs, err = pm1.LastReportedStatuses()
	require.NoError(t, err)
	require.True(t, proto.Equal(statuses, lrs))

	// Statuses are persisted across package manager instances.
	pm2 := initPackageManager(t, tmpDir)

	lrs, err = pm2.LastReportedStatuses()
	require.NoError(t, err)
	require.True(t, proto.Equal(statuses, lrs))
}

func TestPackageManager_UpdateContent(t *testing.T) {
	newAgent := []byte("new agent binary")
	newAgentHash := sha256.Sum256(newAgent)

	t.Run("non-agent package", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		err := pm.UpdateContent(t.Context(), "random-package", "", nil, nil, nil)
		require.EqualError(t, err, "package does not exist")
	})

	t.Run("package too large", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		pm.maxPackageBytes = int64(len(newAgent)) - 1
		err := pm.UpdateContent(t.Context(), agentPackageKey, "", bytes.NewReader(newAgent), newAgentHash[:], nil)
		require.ErrorContains(t, err, "package exceeds maximum size")
	})

	t.Run("invalid hash", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		err := pm.UpdateContent(t.Context(), agentPackageKey, "", bytes.NewReader(newAgent), []byte{0x01, 0x02}, nil)
		require.ErrorContains(t, err, "could not verify package integrity")
	})

	t.Run("verifier cannot be created", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		pm.newVerifier = func(config.Verifier) (verifier.Verifier, error) { return nil, errors.New("no trusted root") }
		err := pm.UpdateContent(t.Context(), agentPackageKey, "", bytes.NewReader(newAgent), newAgentHash[:], nil)
		require.ErrorContains(t, err, "create package verifier: no trusted root")
	})

	t.Run("invalid signature", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		pm.newVerifier = func(config.Verifier) (verifier.Verifier, error) { return failingVerifier{}, nil }
		err := pm.UpdateContent(t.Context(), agentPackageKey, "", bytes.NewReader(newAgent), newAgentHash[:], nil)
		require.ErrorContains(t, err, "could not verify package signature: bad signature")
	})

	t.Run("binary missing from archive", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		pkg := tarGz(t, map[string][]byte{"README": []byte("no binary here")})
		pkgHash := sha256.Sum256(pkg)
		err := pm.UpdateContent(t.Context(), agentPackageKey, "https://example.com/otelcol.tar.gz", bytes.NewReader(pkg), pkgHash[:], nil)
		require.ErrorContains(t, err, "extract agent binary")
	})

	t.Run("install fails", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		var stagedPath string
		pm.am = agentManagerFunc(func(_ context.Context, path string) error {
			stagedPath = path
			return errors.New("agent did not start")
		})
		err := pm.UpdateContent(t.Context(), agentPackageKey, "https://example.com/otelcol", bytes.NewReader(newAgent), newAgentHash[:], nil)
		require.ErrorContains(t, err, "install agent binary: agent did not start")
		require.NoFileExists(t, stagedPath)
	})

	t.Run("raw binary installed", func(t *testing.T) {
		tmpDir := t.TempDir()
		pm := initPackageManager(t, tmpDir)
		pm.am = agentManagerFunc(func(_ context.Context, path string) error {
			require.Equal(t, filepath.Join(tmpDir, "storage", stagedAgentFileName), path)
			by, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, newAgent, by)
			return nil
		})
		err := pm.UpdateContent(t.Context(), agentPackageKey, "https://example.com/otelcol", bytes.NewReader(newAgent), newAgentHash[:], nil)
		require.NoError(t, err)
	})

	t.Run("tar.gz binary installed", func(t *testing.T) {
		pm := initPackageManager(t, t.TempDir())
		pkg := tarGz(t, map[string][]byte{"README": []byte("docs"), pm.agentBinary: newAgent})
		pkgHash := sha256.Sum256(pkg)
		installed := false
		pm.am = agentManagerFunc(func(_ context.Context, path string) error {
			by, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, newAgent, by)
			installed = true
			return nil
		})
		err := pm.UpdateContent(t.Context(), agentPackageKey, "https://example.com/otelcol.tar.gz?token=abc", bytes.NewReader(pkg), pkgHash[:], nil)
		require.NoError(t, err)
		require.True(t, installed)
	})
}

func TestArchiveFormat(t *testing.T) {
	assert.Equal(t, archive.FormatTarGzip, archiveFormat("https://example.com/otelcol-contrib_0.158.0_linux_amd64.tar.gz"))
	assert.Equal(t, archive.FormatTarGzip, archiveFormat("https://example.com/otelcol.tgz?X-Amz-Signature=abc.tar"))
	assert.Equal(t, archive.FormatNone, archiveFormat("https://example.com/otelcol-contrib"))
	assert.Equal(t, archive.FormatNone, archiveFormat("https://example.com/otelcol.tar.gz.sig"))
	assert.Equal(t, archive.FormatNone, archiveFormat(""))
}

func initPackageManager(t *testing.T, tmpDir string) *packageManager {
	t.Helper()
	agentFile := filepath.Join(tmpDir, "agent")
	storageDir := filepath.Join(tmpDir, "storage")

	require.NoError(t, os.MkdirAll(storageDir, 0o700))
	require.NoError(t, os.WriteFile(agentFile, []byte(testAgentFileContents), 0o600))
	ps, err := loadOrCreatePersistentState(filepath.Join(tmpDir, "persistent_state.yaml"), "", zap.NewNop())
	require.NoError(t, err)

	pm, err := newPackageManager(zap.NewNop(), agentFile, storageDir, "v0.110.0", ps, config.DefaultSupervisor().Agent.Package, nil)
	require.NoError(t, err)
	return pm
}

func tarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o700, Size: int64(len(content))}))
		_, err := tw.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}
