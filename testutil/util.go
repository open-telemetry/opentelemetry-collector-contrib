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

package testutil

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.etcd.io/bbolt"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-log-collection/logger"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
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

// NewTestDatabase will return a new database for testing
func NewTestDatabase(t testing.TB) *bbolt.DB {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Errorf(err.Error())
		t.FailNow()
	}

	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	db, err := bbolt.Open(filepath.Join(tempDir, "test.db"), 0666, nil)
	if err != nil {
		t.Errorf(err.Error())
		t.FailNow()
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// NewBuildContext will return a new build context for testing
func NewBuildContext(t testing.TB) operator.BuildContext {
	return operator.BuildContext{
		Database:  NewTestDatabase(t),
		Logger:    logger.New(zaptest.NewLogger(t).Sugar()),
		Namespace: "$",
	}
}

// Trim removes white space from the lines of a string
func Trim(s string) string {
	lines := strings.Split(s, "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		trimmed = append(trimmed, strings.Trim(line, " \t\n"))
	}

	return strings.Join(trimmed, "\n")
}
