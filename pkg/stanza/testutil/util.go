// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"

import (
	"context"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/collector/extension/experimental/storage"
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

func (p *mockPersister) Batch(ctx context.Context, ops ...storage.Operation) error {
	for _, op := range ops {
		switch op.Type {
		case storage.Get:
			val, err := p.Get(ctx, op.Key)
			if err != nil {
				return err
			}
			op.Value = val
		case storage.Set:
			if err := p.Set(ctx, op.Key, op.Value); err != nil {
				return err
			}
		case storage.Delete:
			if err := p.Delete(ctx, op.Key); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *mockPersister) Close(_ context.Context) error {
	p.dataMux.Lock()
	defer p.dataMux.Unlock()
	p.data = make(map[string][]byte)
	return nil
}

// NewUnscopedMockStorage will return a new persister for testing
func NewUnscopedMockStorage() storage.Client {
	data := make(map[string][]byte)
	return &mockPersister{data: data}
}

func NewMockStorage(scope string) storage.Client {
	return operator.NewScopedStorage(scope, NewUnscopedMockStorage())
}

// NewErrPersister will return a new persister for testing
// which will return an error if any of the specified keys are used
func NewErrPersister(errKeys map[string]error) storage.Client {
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
