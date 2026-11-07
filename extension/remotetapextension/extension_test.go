// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapextension

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func getAvailableLocalAddress(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, ln.Close())
	}()
	return ln.Addr().String()
}

func TestRemoteObserverExtension_StartAndServe(t *testing.T) {
	endpoint := getAvailableLocalAddress(t)

	serverConfig := confighttp.NewDefaultServerConfig()
	serverConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  endpoint,
	}

	cfg := &Config{
		ServerConfig: serverConfig,
	}

	factory := NewFactory()
	ext, err := factory.Create(context.Background(), extensiontest.NewNopSettings(typ), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)

	ctx := context.Background()
	host := componenttest.NewNopHost()

	err = ext.Start(ctx, host)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, ext.Shutdown(ctx))
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	require.Eventually(t, func() bool {
		resp, err := client.Get("http://" + endpoint + "/")
		if err != nil {
			return false
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			return false
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false
		}

		return strings.Contains(string(body), "OpenTelemetry Collector Remote Taps Viewer")
	}, 5*time.Second, 100*time.Millisecond)
}

func TestRemoteObserverExtension_InvalidEndpoint(t *testing.T) {
	serverConfig := confighttp.NewDefaultServerConfig()
	serverConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "invalid-endpoint-without-port",
	}

	cfg := &Config{
		ServerConfig: serverConfig,
	}

	factory := NewFactory()
	ext, err := factory.Create(context.Background(), extensiontest.NewNopSettings(typ), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)

	err = ext.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

func TestRemoteObserverExtension_ShutdownNilServer(t *testing.T) {
	ext := &remoteObserverExtension{}
	require.NoError(t, ext.Shutdown(context.Background()))
}
