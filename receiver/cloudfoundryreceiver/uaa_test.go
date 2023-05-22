// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Make sure we can authenticate with valid credentials
func TestValidAuthentication(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	uaa, err := newUAATokenProvider(
		zap.NewNop(),
		cfg.UAA.LimitedHTTPClientSettings,
		cfg.UAA.Username,
		cfg.UAA.Password)

	require.NoError(t, err)
	require.NotNil(t, uaa)

	// No username or password should still succeed
	uaa, err = newUAATokenProvider(
		zap.NewNop(),
		cfg.UAA.LimitedHTTPClientSettings,
		"",
		"")

	require.NoError(t, err)
	require.NotNil(t, uaa)
}

// Make sure authentication fails with empty URL endpoint
func TestInvalidAuthentication(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	cfg.UAA.LimitedHTTPClientSettings.Endpoint = ""

	uaa, err := newUAATokenProvider(
		zap.NewNop(),
		cfg.UAA.LimitedHTTPClientSettings,
		cfg.UAA.Username,
		cfg.UAA.Password)

	require.EqualError(t, err, "client: missing url")
	require.Nil(t, uaa)
}
