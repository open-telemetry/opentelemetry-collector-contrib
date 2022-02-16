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
	context "context"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

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
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf(err.Error())
		}
	})

	return tempDir
}

// Logger will return a new tesst logger
func Logger(t testing.TB) *zap.SugaredLogger {
	return zaptest.NewLogger(t, zaptest.Level(zapcore.ErrorLevel)).Sugar()
}

type mockPersister struct {
	data    map[string][]byte
	dataMux sync.Mutex
}

func (p *mockPersister) Get(ctx context.Context, k string) ([]byte, error) {
	p.dataMux.Lock()
	defer p.dataMux.Unlock()
	return p.data[k], nil
}

func (p *mockPersister) Set(ctx context.Context, k string, v []byte) error {
	p.dataMux.Lock()
	defer p.dataMux.Unlock()
	p.data[k] = v
	return nil
}

func (p *mockPersister) Delete(ctx context.Context, k string) error {
	p.dataMux.Lock()
	defer p.dataMux.Unlock()
	delete(p.data, k)
	return nil
}

// NewUnscopedMockPersister will return a new persister for testing
func NewUnscopedMockPersister() operator.Persister {
	data := make(map[string][]byte)
	return &mockPersister{data: data}
}

// NewMockPersister will return a new persister for testing
func NewMockPersister(scope string) operator.Persister {
	return operator.NewScopedPersister(scope, NewUnscopedMockPersister())
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
