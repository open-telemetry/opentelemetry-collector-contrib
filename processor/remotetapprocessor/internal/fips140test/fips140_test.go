// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:debug fips140=only

package fips140test

import (
	"bufio"
	"crypto/fips140"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/remotetapprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/remotetapprocessor/internal/metadata"
)

// The request is written by hand so the test needs no SHA-1 of its own. The key
// and accept value are the RFC 6455 section 1.3 example.
func TestHandshakeUnderFIPS140Only(t *testing.T) {
	require.True(t, fips140.Enabled(), "//go:debug fips140=only not applied")

	const endpoint = "localhost:12004"
	factory := remotetapprocessor.NewFactory()
	cfg := factory.CreateDefaultConfig().(*remotetapprocessor.Config)
	cfg.ServerConfig.NetAddr.Endpoint = endpoint
	proc, err := factory.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, proc.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, proc.Shutdown(t.Context())) })

	conn, err := net.Dial("tcp", endpoint)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\n"+
		"Host: %s\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n"+
		"Sec-WebSocket-Version: 13\r\n"+
		"\r\n", endpoint)
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	require.Equal(t, "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=", resp.Header.Get("Sec-WebSocket-Accept"))
}
