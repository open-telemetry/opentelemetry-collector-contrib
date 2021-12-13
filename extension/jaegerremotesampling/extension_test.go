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

package jaegerremotesampling

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestNewExtension(t *testing.T) {
	// test
	e, err := newExtension(createDefaultConfig().(*Config), componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	// verify
	assert.NotNil(t, e)
}

func TestStart(t *testing.T) {
	// prepare
	e, err := newExtension(createDefaultConfig().(*Config), componenttest.NewNopTelemetrySettings())
	require.NotNil(t, e)
	require.NoError(t, err)

	// test and verify
	assert.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))
}

func TestShutdown(t *testing.T) {
	// prepare
	e, err := newExtension(createDefaultConfig().(*Config), componenttest.NewNopTelemetrySettings())
	require.NotNil(t, e)
	require.NoError(t, err)
	require.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))

	// test and verify
	assert.NoError(t, e.Shutdown(context.Background()))
}

func TestFailedToStartHTTPServer(t *testing.T) {
	// prepare
	errBooBoo := errors.New("the server made a boo boo")

	e, err := newExtension(createDefaultConfig().(*Config), componenttest.NewNopTelemetrySettings())
	require.NotNil(t, e)
	require.NoError(t, err)

	e.httpServer = &mockComponent{
		StartFunc: func(_ context.Context, _ component.Host) error {
			return errBooBoo
		},
	}

	// test and verify
	assert.Equal(t, errBooBoo, e.Start(context.Background(), componenttest.NewNopHost()))
}

func TestFailedToShutdownHTTPServer(t *testing.T) {
	// prepare
	errBooBoo := errors.New("the server made a boo boo")

	e, err := newExtension(createDefaultConfig().(*Config), componenttest.NewNopTelemetrySettings())
	require.NotNil(t, e)
	require.NoError(t, err)

	e.httpServer = &mockComponent{
		ShutdownFunc: func(_ context.Context) error {
			return errBooBoo
		},
	}
	require.NoError(t, e.Start(context.Background(), componenttest.NewNopHost()))

	// test and verify
	assert.Equal(t, errBooBoo, e.Shutdown(context.Background()))
}

type mockComponent struct {
	StartFunc    func(_ context.Context, _ component.Host) error
	ShutdownFunc func(_ context.Context) error
}

func (s *mockComponent) Start(ctx context.Context, host component.Host) error {
	if s.StartFunc == nil {
		return nil
	}

	return s.StartFunc(ctx, host)
}

func (s *mockComponent) Shutdown(ctx context.Context) error {
	if s.ShutdownFunc == nil {
		return nil
	}

	return s.ShutdownFunc(ctx)
}
