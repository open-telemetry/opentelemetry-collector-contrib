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

package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/opentelemetry/opentelemetry-log-collection/database"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/helper"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

func TestOffsets(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	databasePath := filepath.Join(tempDir, "logagent.db")
	configPath := filepath.Join(tempDir, "config.yaml")
	ioutil.WriteFile(configPath, []byte{}, 0666)

	// capture stdout
	buf := bytes.NewBuffer([]byte{})
	stdout = buf

	// add an offset to the database
	db, err := database.OpenDatabase(databasePath)
	require.NoError(t, err)
	db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(helper.OffsetsBucket)
		require.NoError(t, err)

		_, err = bucket.CreateBucket([]byte("$.testoperatorid1"))
		require.NoError(t, err)
		_, err = bucket.CreateBucket([]byte("$.testoperatorid2"))
		require.NoError(t, err)
		return nil
	})
	db.Close()

	// check that offsets list actually lists the operator
	offsetsList := NewRootCmd()
	offsetsList.SetArgs([]string{
		"offsets", "list",
		"--database", databasePath,
		"--config", configPath,
	})

	err = offsetsList.Execute()
	require.NoError(t, err)
	require.Equal(t, "$.testoperatorid1\n$.testoperatorid2\n", buf.String())

	// clear the offsets
	offsetsClear := NewRootCmd()
	offsetsClear.SetArgs([]string{
		"offsets", "clear",
		"--database", databasePath,
		"--config", configPath,
		"$.testoperatorid2",
	})

	err = offsetsClear.Execute()
	require.NoError(t, err)

	// Check that offsets list only shows uncleared operator id
	buf.Reset()
	err = offsetsList.Execute()
	require.NoError(t, err)
	require.Equal(t, "$.testoperatorid1\n", buf.String())
}
