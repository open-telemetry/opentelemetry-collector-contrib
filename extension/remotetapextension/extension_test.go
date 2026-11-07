// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapextension

import (
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestServesIndexOnConfiguredEndpoint(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	endpoint := l.Addr().String()
	require.NoError(t, l.Close())

	cfg := createDefaultConfig().(*Config)
	cfg.ServerConfig.NetAddr.Endpoint = endpoint
	ext, err := NewFactory().Create(t.Context(), extensiontest.NewNopSettings(typ), cfg)
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, ext.Shutdown(t.Context())) })

	resp, err := http.Get("http://" + endpoint + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<html")
}

func TestStartFailsWhenEndpointInUse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })

	cfg := createDefaultConfig().(*Config)
	cfg.ServerConfig.NetAddr.Endpoint = l.Addr().String()
	ext, err := NewFactory().Create(t.Context(), extensiontest.NewNopSettings(typ), cfg)
	require.NoError(t, err)
	require.Error(t, ext.Start(t.Context(), componenttest.NewNopHost()))
}
