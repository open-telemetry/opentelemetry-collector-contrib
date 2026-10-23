// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal/metadata"
)

// startTestReceiver starts a yangReceiver on a free local endpoint and returns it, along
// with a func to shut it down. Callers must defer the returned shutdown func.
func startTestReceiver(t *testing.T, cfg *Config, csmr *metricsCapture) (endpoint string, shutdown func()) {
	t.Helper()

	endpoint = getFreeEndpoint(t)
	cfg.ServerConfig.NetAddr.Endpoint = endpoint
	cfg.ServerConfig.NetAddr.Transport = "tcp"

	settings := receiver.Settings{
		ID:                component.NewID(metadata.Type),
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
	}

	ctx := t.Context()
	rcvr := createMetricsReceiver(ctx, settings, cfg, csmr)
	require.NoError(t, rcvr.Start(ctx, componenttest.NewNopHost()))

	return endpoint, func() { assert.NoError(t, rcvr.Shutdown(ctx)) }
}

func sampleInterfaceTelemetry() InterfaceTelemetry {
	return InterfaceTelemetry{
		Name:       "GigabitEthernet0/0/0",
		Statistics: map[string]uint64{"in-octets": 1},
	}
}

func TestStreamSecurity_BlockedClient(t *testing.T) {
	csmr := &metricsCapture{}
	cfg := createDefaultConfig().(*Config)
	cfg.Security.AllowedClients = []string{"10.0.0.0/8"}

	endpoint, shutdown := startTestReceiver(t, cfg, csmr)
	defer shutdown()

	conn := newReadyClientConn(t, endpoint)
	defer conn.Close()

	err := sendInterfaceTelemetryData(conn, "node1", "sub1", "path1", sampleInterfaceTelemetry())
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok, "expected a gRPC status error, got: %v", err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	assert.Empty(t, csmr.GetMetrics(), "a blocked client's telemetry must not reach the consumer")
}

func TestStreamSecurity_AllowedClient(t *testing.T) {
	csmr := &metricsCapture{}
	cfg := createDefaultConfig().(*Config)
	cfg.Security.AllowedClients = []string{"127.0.0.0/8"}

	endpoint, shutdown := startTestReceiver(t, cfg, csmr)
	defer shutdown()

	conn := newReadyClientConn(t, endpoint)
	defer conn.Close()

	err := sendInterfaceTelemetryData(conn, "node1", "sub1", "path1", sampleInterfaceTelemetry())
	require.NoError(t, err)
	assert.Len(t, csmr.GetMetrics(), 1)
}

func TestStreamSecurity_RateLimit(t *testing.T) {
	csmr := &metricsCapture{}
	cfg := createDefaultConfig().(*Config)
	cfg.Security.RateLimiting = RateLimitingConfig{
		Enabled:           true,
		RequestsPerSecond: 1,
		BurstSize:         1,
		CleanupInterval:   time.Minute,
	}

	endpoint, shutdown := startTestReceiver(t, cfg, csmr)
	defer shutdown()

	conn := newReadyClientConn(t, endpoint)
	defer conn.Close()

	// Burst size 1: the first stream open consumes the only available token.
	err := sendInterfaceTelemetryData(conn, "node1", "sub1", "path1", sampleInterfaceTelemetry())
	require.NoError(t, err)

	// A second stream opened immediately after must be rejected; the bucket only
	// refills at 1 token/second.
	err = sendInterfaceTelemetryData(conn, "node1", "sub1", "path1", sampleInterfaceTelemetry())
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok, "expected a gRPC status error, got: %v", err)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
}
