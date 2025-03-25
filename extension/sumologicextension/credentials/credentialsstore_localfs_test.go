// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/api"
)

func TestCredentialsStoreLocalFs(t *testing.T) {
	dir := t.TempDir()

	const key = "my_storage_key"

	creds := CollectorCredentials{
		CollectorName: "name",
		Credentials: api.OpenRegisterResponsePayload{
			CollectorCredentialID:  "credentialId",
			CollectorCredentialKey: "credentialKey",
			CollectorID:            "id",
		},
	}

	sut := LocalFsStore{
		collectorCredentialsDirectory: dir,
		logger:                        zap.NewNop(),
	}

	require.NoError(t, sut.Store(key, creds))

	require.True(t, sut.Check(key))

	actual, err := sut.Get(key)
	require.NoError(t, err)
	assert.Equal(t, creds, actual)

	require.NoError(t, sut.Delete(key))
	// Make sure the file got deleted and there is nothing in the credentials store dir.
	var fileCounter int
	require.NoError(t,
		filepath.WalkDir(dir,
			func(_ string, d fs.DirEntry, _ error) error {
				if d.IsDir() {
					return nil
				}
				fileCounter++
				return nil
			},
		),
	)
	require.EqualValues(t, 0, fileCounter)
}

func TestCredentialsStoreValidate(t *testing.T) {
	var expectedFileMode fs.FileMode
	dir := filepath.Join(t.TempDir(), "store")
	if runtime.GOOS == "windows" { // on Windows, we get 0777 for writable directories
		expectedFileMode = fs.FileMode(0o777)
	} else {
		expectedFileMode = fs.FileMode(0o700)
	}
	err := os.Mkdir(dir, 0o400)
	require.NoError(t, err)

	store, err := NewLocalFsStore(WithCredentialsDirectory(dir), WithLogger(zap.NewNop()))
	require.NoError(t, err)
	err = store.Validate()
	require.NoError(t, err)

	stat, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, expectedFileMode.Perm(), stat.Mode().Perm())
}
