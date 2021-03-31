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

package helper

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/database"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestPersisterCache(t *testing.T) {
	stubDatabase := database.NewStubDatabase()
	persister := NewScopedDBPersister(stubDatabase, "test")
	persister.Set("key", []byte("value"))
	value := persister.Get("key")
	require.Equal(t, []byte("value"), value)
}

func TestPersisterLoad(t *testing.T) {
	tempDir := testutil.NewTempDir(t)
	db, err := database.OpenDatabase(filepath.Join(tempDir, "test.db"))
	require.NoError(t, err)
	persister := NewScopedDBPersister(db, "test")
	persister.Set("key", []byte("value"))

	err = persister.Sync()
	require.NoError(t, err)

	newPersister := NewScopedDBPersister(db, "test")
	err = newPersister.Load()
	require.NoError(t, err)

	value := newPersister.Get("key")
	require.Equal(t, []byte("value"), value)
}
