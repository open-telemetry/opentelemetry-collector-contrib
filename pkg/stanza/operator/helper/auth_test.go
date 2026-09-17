// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/extension/extensionauth"
)

var _ extensionauth.Server = mockServerAuth{}

type mockServerAuth struct {
	component.StartFunc
	component.ShutdownFunc
}

func (mockServerAuth) Authenticate(ctx context.Context, _ map[string][]string) (context.Context, error) {
	return ctx, nil
}

type mockHost struct {
	extensions map[component.ID]component.Component
}

func (h mockHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

func TestGetServer(t *testing.T) {
	authID := component.MustNewID("auth")
	authServer := mockServerAuth{}

	cases := []struct {
		name      string
		cfg       *AuthConfig
		host      component.Host
		gotServer extensionauth.Server
		gotError  error
	}{
		{
			name: "empty",
			cfg:  &AuthConfig{},
			host: mockHost{},
		},
		{
			name:     "configured but no host",
			cfg:      &AuthConfig{Config: configauth.Config{AuthenticatorID: authID}},
			gotError: errNoHost,
		},
		{
			name:      "resolves server authenticator",
			cfg:       &AuthConfig{Config: configauth.Config{AuthenticatorID: authID}},
			host:      mockHost{extensions: map[component.ID]component.Component{authID: authServer}},
			gotServer: authServer,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server, err := tc.cfg.GetServer(t.Context(), tc.host)
			if tc.gotError != nil {
				require.ErrorIs(t, err, tc.gotError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.gotServer, server)
		})
	}
}
