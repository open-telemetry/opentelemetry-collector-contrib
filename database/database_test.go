// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package database

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// NewTempDir will return a new temp directory for testing
func NewTempDir(t testing.TB) string {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Errorf(err.Error())
		t.FailNow()
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return tempDir
}

func TestOpenDatabase(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		tempDir := NewTempDir(t)
		db, err := OpenDatabase(filepath.Join(tempDir, "test.db"))
		require.NoError(t, err)
		require.NotNil(t, db)
	})

	t.Run("NoFile", func(t *testing.T) {
		db, err := OpenDatabase("")
		require.NoError(t, err)
		require.NotNil(t, db)
		require.IsType(t, &StubDatabase{}, db)
	})

	t.Run("NonexistantPathIsCreated", func(t *testing.T) {
		tempDir := NewTempDir(t)
		db, err := OpenDatabase(filepath.Join(tempDir, "nonexistdir", "test.db"))
		require.NoError(t, err)
		require.NotNil(t, db)
	})

	t.Run("BadPermissions", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not have the same kind of file permissions")
		}
		tempDir := NewTempDir(t)
		err := os.MkdirAll(filepath.Join(tempDir, "badperms"), 0666)
		require.NoError(t, err)
		db, err := OpenDatabase(filepath.Join(tempDir, "badperms", "nonexistdir", "test.db"))
		require.Error(t, err)
		require.Nil(t, db)
	})

	t.Run("ExecuteOnlyPermissions", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not have the same kind of file permissions")
		}
		tempDir := NewTempDir(t)
		err := os.MkdirAll(filepath.Join(tempDir, "badperms"), 0111)
		require.NoError(t, err)
		db, err := OpenDatabase(filepath.Join(tempDir, "badperms", "nonexistdir", "test.db"))
		require.Error(t, err)
		require.Nil(t, db)
	})
}

func TestStubDatabase(t *testing.T) {
	stubDatabase := NewStubDatabase()
	err := stubDatabase.Close()
	require.NoError(t, err)

	err = stubDatabase.Sync()
	require.NoError(t, err)

	err = stubDatabase.Update(nil)
	require.NoError(t, err)

	err = stubDatabase.View(nil)
	require.NoError(t, err)
}
