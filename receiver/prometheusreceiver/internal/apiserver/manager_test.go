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
)

func newTestManager(t *testing.T, cfg *Config) *Manager {
	t.Helper()

	require.NotNil(t, cfg)

	settings := receivertest.NewNopSettings(metadata.Type)
	registry := prometheus.NewRegistry()
	promCfg := &promconfig.Config{}

	return NewManager(settings, cfg, promCfg, registry, registry, nil)
}

func TestNewManagerInitializesFields(t *testing.T) {
	cfg := &Config{}

	manager := newTestManager(t, cfg)

	assert.Equal(t, cfg, manager.cfg)
	assert.NotNil(t, manager.promCfg)
	assert.NotNil(t, manager.shutdown)
	assert.NotNil(t, manager.registry)
	assert.NotNil(t, manager.registerer)
}

func TestNewManagerReturnsNilWithoutConfig(t *testing.T) {
	settings := receivertest.NewNopSettings(metadata.Type)
	registry := prometheus.NewRegistry()
	promCfg := &promconfig.Config{}

	manager := NewManager(settings, nil, promCfg, registry, registry, nil)

	require.Nil(t, manager)
}

func TestManagerApplyConfigAndGetConfig(t *testing.T) {
	manager := newTestManager(t, &Config{})
	newCfg := &promconfig.Config{
		GlobalConfig: promconfig.DefaultGlobalConfig,
	}

	require.NoError(t, manager.ApplyConfig(newCfg))
	assert.Equal(t, newCfg, manager.GetConfig())
}

func TestManagerShutdownClosesChannelAndServer(t *testing.T) {
	manager := newTestManager(t, &Config{})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &http.Server{}

	done := make(chan struct{})
	go func() {
		_ = server.Serve(listener)
		close(done)
	}()

	manager.server = server

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

func TestManagerStartInvalidEndpoint(t *testing.T) {
	cfg := &Config{
		ServerConfig: confighttp.ServerConfig{
			NetAddr: confignet.AddrConfig{
				Endpoint: "localhost", // missing port forces ToListener error
			},
		},
	}

	manager := newTestManager(t, cfg)

	err := manager.Start(context.Background(), componenttest.NewNopHost(), nil)
	require.Error(t, err)
}
