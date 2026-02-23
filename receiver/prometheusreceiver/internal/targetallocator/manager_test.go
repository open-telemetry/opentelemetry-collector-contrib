// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/promslog"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

func TestNewManager(t *testing.T) {
	cfg := &Config{
		Interval:    30 * time.Second,
		CollectorID: "test-collector",
	}
	promCfg := &promconfig.Config{
		ScrapeConfigs: []*promconfig.ScrapeConfig{
			{JobName: "test-job"},
		},
	}

	manager := NewManager(receivertest.NewNopSettings(metadata.Type), cfg, promCfg)

	assert.NotNil(t, manager)
	assert.Equal(t, cfg, manager.cfg)
	assert.Equal(t, promCfg, manager.promCfg)
	assert.NotNil(t, manager.shutdown)
	assert.NotNil(t, manager.configUpdateCount)
	assert.Len(t, manager.initialScrapeConfigs, 1)
}

func TestManagerShutdown(t *testing.T) {
	// Create a mock target allocator server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/scrape_configs" {
			_, _ = w.Write([]byte("{}"))
		}
	}))
	defer server.Close()

	cfg := &Config{
		Interval:    100 * time.Millisecond,
		CollectorID: "test-collector",
	}
	cfg.Endpoint = server.URL
	promCfg, err := promconfig.Load("", nil)
	require.NoError(t, err)

	// Create a logger with observer to capture logs
	core, logs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	manager := NewManager(settings, cfg, promCfg)

	// Start the manager so the goroutine is running
	ctx := t.Context()

	// Initialize Prometheus managers using the same pattern as manager_test.go
	promLogger := promslog.NewNopLogger()
	reg := prometheus.NewRegistry()
	sdMetrics, err := discovery.RegisterSDMetrics(reg, discovery.NewRefreshMetrics(reg))
	require.NoError(t, err)
	discoveryManager := discovery.NewManager(ctx, promLogger, reg, sdMetrics)
	require.NotNil(t, discoveryManager)

	scrapeManager, err := scrape.NewManager(&scrape.Options{}, promLogger, nil, nil, reg)
	require.NoError(t, err)
	defer scrapeManager.Stop()

	err = manager.Start(ctx, componenttest.NewNopHost(), scrapeManager, discoveryManager)
	require.NoError(t, err)

	// Shutdown the manager
	manager.Shutdown()

	// Wait for the shutdown to complete with a timeout
	require.Eventually(t, func() bool {
		// Check if the log message "Stopping target allocator" was logged
		logEntries := logs.FilterMessage("Stopping target allocator")
		return logEntries.Len() > 0
	}, 5*time.Second, 50*time.Millisecond, "expected shutdown log message")
}

func TestInstantiateShard(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		envVar   string
		expected []byte
		setEnv   bool
	}{
		{
			name:     "shard environment variable set",
			input:    []byte("replica-$(SHARD)"),
			envVar:   "5",
			expected: []byte("replica-5"),
			setEnv:   true,
		},
		{
			name:     "shard environment variable not set",
			input:    []byte("replica-$(SHARD)"),
			expected: []byte("replica-0"),
			setEnv:   false,
		},
		{
			name:     "multiple shard placeholders",
			input:    []byte("job-$(SHARD)-replica-$(SHARD)"),
			envVar:   "3",
			expected: []byte("job-3-replica-3"),
			setEnv:   true,
		},
		{
			name:     "no shard placeholder",
			input:    []byte("static-config"),
			expected: []byte("static-config"),
			setEnv:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lookup func(string) (string, bool)
			if tt.setEnv {
				lookup = func(_ string) (string, bool) {
					return tt.envVar, true
				}
			} else {
				lookup = func(_ string) (string, bool) {
					return "", false
				}
			}

			result := instantiateShard(tt.input, lookup)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetScrapeConfigsResponse_InvalidURL(t *testing.T) {
	httpClient := &http.Client{}
	invalidURL := "://invalid-url"

	_, err := getScrapeConfigsResponse(httpClient, invalidURL)
	assert.Error(t, err)
}

func TestGetScrapeConfigsResponse_HTTPError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	httpClient := &http.Client{}

	config, err := getScrapeConfigsResponse(httpClient, server.URL)
	// This should succeed in reading the response but return empty config
	assert.NoError(t, err)
	assert.Empty(t, config)
}

func TestGetScrapeConfigsResponse_InvalidYAML(t *testing.T) {
	// Create a test server that returns invalid YAML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid: yaml: content: [[["))
	}))
	defer server.Close()

	httpClient := &http.Client{}

	_, err := getScrapeConfigsResponse(httpClient, server.URL)
	assert.Error(t, err)
}
