// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"

import (
	"context"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// Logger will return a new tesst logger
func Logger(t testing.TB) *zap.SugaredLogger {
	return zaptest.NewLogger(t, zaptest.Level(zapcore.ErrorLevel)).Sugar()
}

type mockPersister struct {
	data    map[string][]byte
	dataMux sync.Mutex
	errKeys map[string]error
}

func (p *mockPersister) Get(_ context.Context, k string) ([]byte, error) {
	p.dataMux.Lock()
	defer p.dataMux.Unlock()
	if _, ok := p.errKeys[k]; ok {
		return nil, p.errKeys[k]
	}
	return p.data[k], nil
}

func (p *mockPersister) Set(_ context.Context, k string, v []byte) error {
	p.dataMux.Lock()
	defer p.dataMux.Unlock()
	if _, ok := p.errKeys[k]; ok {
		return p.errKeys[k]
	}
	p.data[k] = v
	return nil
}

func (p *mockPersister) Delete(_ context.Context, k string) error {
	p.dataMux.Lock()
	defer p.dataMux.Unlock()
	if _, ok := p.errKeys[k]; ok {
		return p.errKeys[k]
	}
	delete(p.data, k)
	return nil
}

// NewUnscopedMockPersister will return a new persister for testing
func NewUnscopedMockPersister() operator.Persister {
	data := make(map[string][]byte)
	return &mockPersister{data: data}
}

func NewMockPersister(scope string) operator.Persister {
	return operator.NewScopedPersister(scope, NewUnscopedMockPersister())
}

// NewErrPersister will return a new persister for testing
// which will return an error if any of the specified keys are used
func NewErrPersister(errKeys map[string]error) operator.Persister {
	data := make(map[string][]byte)
	return &mockPersister{data: data, errKeys: errKeys}
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
