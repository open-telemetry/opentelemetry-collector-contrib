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

package filereloadereceiver

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFileWatcher(t *testing.T) {
	td := t.TempDir()
	fw := newFileWatcher(filepath.Join(td, "*.yml"))

	l, b, e := fw.List()
	require.NoError(t, e)
	require.False(t, b)
	require.Len(t, l, 0)

	p1 := filepath.Join(td, "1.yml")
	createFile(t, p1, "1")

	// Creating a yaml file should result in an updated=true with the file in the list
	l, b, e = fw.List()
	require.NoError(t, e)
	require.True(t, b)
	require.Len(t, l, 1)
	require.Equal(t, p1, l[0])

	// Creating a non yaml file should result in updated=false with the same list as last time
	p2 := filepath.Join(td, "2.txt")
	createFile(t, p2, "123")
	l, b, e = fw.List()
	require.NoError(t, e)
	require.False(t, b)
	require.Len(t, l, 1)

	// Deleting a non yaml file should result in updated=false
	deleteFile(t, p2)
	l, b, e = fw.List()
	require.NoError(t, e)
	require.False(t, b)
	require.Len(t, l, 1)

	// Creating a second file should result in updated=true with 2 files in the list and should be in sorted order
	p2 = filepath.Join(td, "2.yml")
	createFile(t, p2, "2")
	l, b, e = fw.List()
	require.NoError(t, e)
	require.True(t, b)
	require.Len(t, l, 2)
	require.Equal(t, p2, l[1])

	// Wait before modifying p2 so that the mod time will be different from the last check.
	// We're assuming not all platforms support sub-second resolution.
	time.Sleep(time.Second - time.Now().Sub(fw.lastCheck))

	// Modifying p2 should cause updated=true
	createFile(t, p2, "3")
	l, b, e = fw.List()
	require.NoError(t, e)
	require.True(t, b)
	require.Len(t, l, 2)
	require.Equal(t, p2, l[1])
	require.Equal(t, p1, l[0])

	// deleting a file should result in updated=true and return only appropriate files
	deleteFile(t, p1)
	l, b, e = fw.List()
	require.NoError(t, e)
	require.True(t, b)
	require.Len(t, l, 1)
	require.Equal(t, p2, l[0])

	// no changes to the file system should result in updated=false with the same list as before
	l, b, e = fw.List()
	require.NoError(t, e)
	require.False(t, b)
	require.Len(t, l, 1)
	require.Equal(t, p2, l[0])
}

func createFile(t *testing.T, path, data string) {
	f, err := os.Create(path)
	require.NoError(t, err)
	_, err = f.WriteString(data)
	require.NoError(t, err)
	require.NoError(t, f.Sync())
	require.NoError(t, f.Close())
}

func deleteFile(t *testing.T, path string) {
	require.Nil(t, os.Remove(path))
}
