// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileaddon

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/addon"
)

func install(t *testing.T, content []byte, downloadURL string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	err := NewFactory().Install(t.Context(), addon.InstallSettings{
		ID:          addon.ID{Type: Type, Name: "geoip"},
		Dir:         dir,
		Content:     content,
		DownloadURL: downloadURL,
		Logger:      zap.NewNop(),
	})
	return dir, err
}

// TestInstallRawFile covers serving a file as-is, which is how a GeoIP
// database would be offered without repackaging it.
func TestInstallRawFile(t *testing.T) {
	dir, err := install(t, []byte("mmdb"), "https://example.invalid/db/GeoLite2-City.mmdb?token=x")
	require.NoError(t, err)

	body, err := os.ReadFile(filepath.Join(dir, "GeoLite2-City.mmdb"))
	require.NoError(t, err)
	assert.Equal(t, "mmdb", string(body))
}

func TestInstallTarGz(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "GeoLite2-City.mmdb", Mode: 0o600, Size: 4, Typeflag: tar.TypeReg}))
	_, err := tw.Write([]byte("mmdb"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	for _, suffix := range []string{".tar.gz", ".tgz"} {
		t.Run(suffix, func(t *testing.T) {
			dir, err := install(t, buf.Bytes(), "https://example.invalid/geoip"+suffix)
			require.NoError(t, err)
			assert.FileExists(t, filepath.Join(dir, "GeoLite2-City.mmdb"))
			assert.NoFileExists(t, filepath.Join(dir, "geoip"+suffix), "an archive must be extracted, not stored")
		})
	}
}

func TestInstallRejectsURLWithoutFileName(t *testing.T) {
	for _, u := range []string{"https://example.invalid/", "https://example.invalid", "::not a url"} {
		t.Run(u, func(t *testing.T) {
			_, err := install(t, []byte("x"), u)
			require.Error(t, err)
		})
	}
}

func TestFileAddonRestartsCollectorOnceAndRejectsConfig(t *testing.T) {
	a, err := NewFactory().Create(t.Context(), addon.Settings{ID: addon.ID{Type: Type, Name: "geoip"}, Dir: t.TempDir(), Logger: zap.NewNop()})
	require.NoError(t, err)
	ctx := t.Context()

	effect, err := a.Configure(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, addon.Effect{}, effect)

	// The first start asks for a Collector restart so the Collector reads the new
	// files; later starts change nothing.
	effect, err = a.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, addon.Effect{RestartAgent: true}, effect)

	effect, err = a.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, addon.Effect{}, effect)

	_, err = a.Configure(ctx, []byte("anything"))
	require.ErrorContains(t, err, "does not accept configuration")

	require.NoError(t, a.Shutdown(ctx))
}
