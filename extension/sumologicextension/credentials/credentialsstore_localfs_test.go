// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	"github.com/SumoLogic/sumologic-otel-collector/pkg/extension/sumologicextension/api"
)

func TestCredentialsStoreLocalFs(t *testing.T) {
	dir, err := os.MkdirTemp("", "otelcol-sumo-credentials-store-local-fs-test-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	const key = "my_storage_key"

	creds := CollectorCredentials{
		CollectorName: "name",
		Credentials: api.OpenRegisterResponsePayload{
			CollectorCredentialId:  "credentialId",
			CollectorCredentialKey: "credentialKey",
			CollectorId:            "id",
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
			func(path string, d fs.DirEntry, err error) error {
				if d.IsDir() {
					return nil
				}
				fileCounter++
				return nil
			},
		),
	)
	require.EqualValues(t, fileCounter, 0)
}

func TestCredentialsStoreValidate(t *testing.T) {
	var expectedFileMode fs.FileMode
	dir := filepath.Join(t.TempDir(), "store")
	if runtime.GOOS == "windows" { // on Windows, we get 0777 for writable directories
		expectedFileMode = fs.FileMode(0777)
	} else {
		expectedFileMode = fs.FileMode(0700)
	}
	err := os.Mkdir(dir, 0400)
	require.NoError(t, err)

	store, err := NewLocalFsStore(WithCredentialsDirectory(dir), WithLogger(zap.NewNop()))
	require.NoError(t, err)
	err = store.Validate()
	require.NoError(t, err)

	stat, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, expectedFileMode.Perm(), stat.Mode().Perm())
}
