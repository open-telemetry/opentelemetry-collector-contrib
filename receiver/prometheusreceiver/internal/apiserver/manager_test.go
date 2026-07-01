// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/apiserver"

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/sharedpromconfig"
)

func TestNewManager(t *testing.T) {
	cfg := &Config{}
	registry := prometheus.NewRegistry()
	promCfg := sharedpromconfig.NewConfig(&promconfig.Config{
		ScrapeConfigs: []*promconfig.ScrapeConfig{
			{JobName: "test-job"},
		},
	})
	manager := NewManager(
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		promCfg,
		registry,
		registry,
	)

	assert.NotNil(t, manager)
	assert.Equal(t, cfg, manager.cfg)
	assert.Equal(t, promCfg, manager.promCfg)
	assert.NotNil(t, manager.shutdown)
	assert.NotNil(t, manager.registry)
	assert.NotNil(t, manager.registerer)
}

func TestManagerShutdown(t *testing.T) {
	registry := prometheus.NewRegistry()
	cfg := &Config{}

	manager := NewManager(
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		sharedpromconfig.NewConfig(&promconfig.Config{}),
		registry,
		registry,
	)
	require.NotNil(t, manager)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &http.Server{ReadHeaderTimeout: time.Second}

	done := make(chan struct{})
	go func() {
		_ = server.Serve(listener)
		close(done)
	}()

	manager.server = server

	// Cancel test if shutdown takes too long
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	require.NoError(t, manager.Shutdown(ctx))

	select {
	case <-manager.shutdown:
	default:
		t.Fatal("shutdown channel not closed")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("http server did not exit")
	}
}

func TestManagerStart(t *testing.T) {
	tests := []struct {
		name      string
		manager   *Manager
		expectErr string
	}{
		{
			name: "invalid endpoint",
			manager: NewManager(
				receivertest.NewNopSettings(metadata.Type),
				&Config{
					ServerConfig: confighttp.ServerConfig{
						NetAddr: confignet.AddrConfig{
							Endpoint: "localhost",
						},
					},
				},
				sharedpromconfig.NewConfig(&promconfig.Config{}),
				prometheus.NewRegistry(),
				prometheus.NewRegistry(),
			),
			expectErr: "failed to create listener",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manager.Start(t.Context(), componenttest.NewNopHost(), nil)
			if tt.expectErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectErr)
		})
	}
}
