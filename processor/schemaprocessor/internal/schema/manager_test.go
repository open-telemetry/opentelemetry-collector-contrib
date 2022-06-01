// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schema

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewManager(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		opts     []ManagerOption
		err      error
	}{
		{scenario: "Default manager", err: nil},
		{scenario: "Provided logger", opts: []ManagerOption{WithManagerLogger(zaptest.NewLogger(t))}, err: nil},
		{scenario: "Provided http client", opts: []ManagerOption{WithManagerHTTPClient(http.DefaultClient)}, err: nil},
		{scenario: "Provided nil logger", opts: []ManagerOption{WithManagerLogger(nil)}, err: errNilValueProvided},
		{scenario: "Provided nil http client", opts: []ManagerOption{WithManagerHTTPClient(nil)}, err: errNilValueProvided},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			m, err := NewManager(tc.opts...)
			assert.ErrorIs(t, err, tc.err, "Must match the expected error")
			if tc.err != nil {
				return
			}
			assert.NotNil(t, m, "Must have a non nil client")
		})
	}
}

func TestRequestSchema(t *testing.T) {
	t.Parallel()

	const (
		schemaURL = "http://opentelemetry.io/schemas/1.9.0"
	)

	m, err := NewManager(
		WithManagerLogger(zaptest.NewLogger(t)),
	)
	require.NoError(t, err, "Must not error when created manager")
	require.NotNil(t, m, "Must have a valid client")

	nop, ok := m.RequestSchema("/not/a/valid/schema/URL").(NoopSchema)
	require.True(t, ok, "Must return a NoopSchema if no valid schema URL is provided")
	require.NotNil(t, nop, "Must have a valid schema")

	s, ok := m.RequestSchema(schemaURL).(*translation)
	require.True(t, ok, "Can cast to the concrete type")
	require.NotNil(t, s, "Must have a valid schema")

	assert.False(t, s.rw.TryRLock(), "The object must be locked for reading")

	var wg sync.WaitGroup
	ctx, done := context.WithCancel(context.Background())
	t.Cleanup(done)

	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()
		assert.NoError(t, m.ProcessRequests(ctx), "Must not error when shutdown correctly")
	}(ctx)

	assert.True(t, s.SupportedVersion(&Version{1, 2, 0}), "Must have the version listed as supported")
	done()

	wg.Wait()

	s, ok = m.RequestSchema(schemaURL).(*translation)
	require.True(t, ok, "Can cast to the concrete type")
	require.NotNil(t, s, "Must have a valid schema")
}
