// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcp

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configauth"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type mockServerAuthenticator struct {
	authenticateFunc func(ctx context.Context, headers map[string][]string) (context.Context, error)
}

func (m *mockServerAuthenticator) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (m *mockServerAuthenticator) Shutdown(ctx context.Context) error {
	return nil
}

func (m *mockServerAuthenticator) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	if m.authenticateFunc != nil {
		return m.authenticateFunc(ctx, headers)
	}
	return ctx, nil
}

type mockHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	return m.extensions
}

func TestTCPInputAuth(t *testing.T) {
	mockAuthID := component.MustNewIDWithName("mockauth", "server")

	t.Run("SuccessfulAuthentication", func(t *testing.T) {
		cfg := NewConfigWithID("test_id")
		cfg.ListenAddress = "127.0.0.1:0"
		cfg.Auth = &configauth.Config{
			AuthenticatorID: mockAuthID,
		}

		set := componenttest.NewNopTelemetrySettings()
		op, err := cfg.Build(set)
		require.NoError(t, err)

		mockOutput := testutil.Operator{}
		tcpInput := op.(*Input)
		tcpInput.OutputOperators = []operator.Operator{&mockOutput}

		authCalled := false
		mockAuth := &mockServerAuthenticator{
			authenticateFunc: func(ctx context.Context, headers map[string][]string) (context.Context, error) {
				authCalled = true
				clientInfo := client.FromContext(ctx)
				assert.NotNil(t, clientInfo.Addr)
				
				// Inject auth data to verify context propagation
				newCtx := client.NewContext(ctx, client.Info{
					Addr: clientInfo.Addr,
					Auth: &mockAuthData{username: "testuser"},
				})
				return newCtx, nil
			},
		}

		host := &mockHost{
			extensions: map[component.ID]component.Component{
				mockAuthID: mockAuth,
			},
		}
		tcpInput.SetHost(host)

		entryChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			// Check if the auth data is present in the context passed to Process
			ctx := args.Get(0).(context.Context)
			clientInfo := client.FromContext(ctx)
			assert.NotNil(t, clientInfo.Auth)
			assert.Equal(t, "testuser", clientInfo.Auth.GetAttribute("username"))

			entryChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		err = tcpInput.Start(testutil.NewUnscopedMockPersister())
		require.NoError(t, err)
		defer func() {
			require.NoError(t, tcpInput.Stop())
		}()

		// Find the resolved address port
		addr := tcpInput.listener.Addr().String()

		conn, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		defer conn.Close()

		_, err = conn.Write([]byte("testlog\n"))
		require.NoError(t, err)

		select {
		case e := <-entryChan:
			assert.Equal(t, "testlog", e.Body)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for log entry")
		}

		assert.True(t, authCalled)
	})

	t.Run("FailedAuthentication", func(t *testing.T) {
		cfg := NewConfigWithID("test_id")
		cfg.ListenAddress = "127.0.0.1:0"
		cfg.Auth = &configauth.Config{
			AuthenticatorID: mockAuthID,
		}

		set := componenttest.NewNopTelemetrySettings()
		op, err := cfg.Build(set)
		require.NoError(t, err)

		mockOutput := testutil.Operator{}
		tcpInput := op.(*Input)
		tcpInput.OutputOperators = []operator.Operator{&mockOutput}

		authCalled := false
		mockAuth := &mockServerAuthenticator{
			authenticateFunc: func(ctx context.Context, headers map[string][]string) (context.Context, error) {
				authCalled = true
				return ctx, errors.New("invalid credentials")
			},
		}

		host := &mockHost{
			extensions: map[component.ID]component.Component{
				mockAuthID: mockAuth,
			},
		}
		tcpInput.SetHost(host)

		// Process should not be called
		mockOutput.On("Process", mock.Anything, mock.Anything).Return(nil)

		err = tcpInput.Start(testutil.NewUnscopedMockPersister())
		require.NoError(t, err)
		defer func() {
			require.NoError(t, tcpInput.Stop())
		}()

		addr := tcpInput.listener.Addr().String()

		conn, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		defer conn.Close()

		_, err = conn.Write([]byte("testlog\n"))
		require.NoError(t, err)

		// Wait a bit to ensure no message is processed
		time.Sleep(500 * time.Millisecond)

		assert.True(t, authCalled)
		mockOutput.AssertNotCalled(t, "Process", mock.Anything, mock.Anything)
	})
}

type mockAuthData struct {
	username string
}

func (m *mockAuthData) GetAttribute(name string) any {
	if name == "username" {
		return m.username
	}
	return nil
}

func (m *mockAuthData) GetAttributeNames() []string {
	return []string{"username"}
}
