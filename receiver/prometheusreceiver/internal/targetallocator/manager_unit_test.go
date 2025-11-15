// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package targetallocator

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	promconfig "github.com/prometheus/prometheus/config"
	"go.opentelemetry.io/collector/receiver/receivertest"

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

	manager := NewManager(receivertest.NewNopSettings(metadata.Type), cfg, promCfg, true)

	assert.NotNil(t, manager)
	assert.Equal(t, cfg, manager.cfg)
	assert.Equal(t, promCfg, manager.promCfg)
	assert.True(t, manager.enableNativeHistograms)
	assert.NotNil(t, manager.shutdown)
	assert.NotNil(t, manager.configUpdateCount)
	assert.Len(t, manager.initialScrapeConfigs, 1)
}

func TestShutdown(t *testing.T) {
	cfg := &Config{
		Interval:    30 * time.Second,
		CollectorID: "test-collector",
	}
	promCfg, err := promconfig.Load("", nil)
	require.NoError(t, err)

	manager := NewManager(receivertest.NewNopSettings(metadata.Type), cfg, promCfg, false)

	// Shutdown should close the channel
	manager.Shutdown()

	// Verify the channel is closed
	select {
	case <-manager.shutdown:
		// Channel is closed, test passes
	default:
		t.Fatal("shutdown channel was not closed")
	}
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
			// Clean up environment
			if tt.setEnv {
				t.Setenv("SHARD", tt.envVar)
			} else {
				os.Unsetenv("SHARD")
			}

			result := instantiateShard(tt.input)
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	httpClient := &http.Client{}

	_, err := getScrapeConfigsResponse(httpClient, server.URL)
	// This should succeed in reading the response but return empty config
	assert.NoError(t, err)
}

func TestGetScrapeConfigsResponse_InvalidYAML(t *testing.T) {
	// Create a test server that returns invalid YAML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid: yaml: content: [[["))
	}))
	defer server.Close()

	httpClient := &http.Client{}

	_, err := getScrapeConfigsResponse(httpClient, server.URL)
	assert.Error(t, err)
}

func TestGetScrapeConfigHash_ErrorCase(t *testing.T) {
	// Test hash consistency with empty config
	emptyConfig := map[string]*promconfig.ScrapeConfig{}

	hash1, err := getScrapeConfigHash(emptyConfig)
	require.NoError(t, err)

	hash2, err := getScrapeConfigHash(emptyConfig)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2)
}
