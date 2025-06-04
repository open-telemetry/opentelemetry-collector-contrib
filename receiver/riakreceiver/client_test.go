// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package riakreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver/internal/model"
)

const (
	statsAPIResponseFile = "get_stats_response.json"
)

func TestNewClient(t *testing.T) {
	clientConfigNonexistentCA := confighttp.NewDefaultClientConfig()
	clientConfigNonexistentCA.Endpoint = defaultEndpoint
	clientConfigNonexistentCA.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "/non/existent",
		},
	}

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint

	testCase := []struct {
		desc        string
		cfg         *Config
		expectError error
	}{
		{
			desc: "Invalid HTTP config",
			cfg: &Config{
				ClientConfig: clientConfigNonexistentCA,
			},
			expectError: errors.New("failed to create HTTP Client"),
		},
		{
			desc: "Valid Configuration",
			cfg: &Config{
				ClientConfig: clientConfig,
			},
			expectError: nil,
		},
	}

	for _, tc := range testCase {
		t.Run(tc.desc, func(t *testing.T) {
			ac, err := newClient(context.Background(), tc.cfg, componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings(), zap.NewNop())
			if tc.expectError != nil {
				require.Nil(t, ac)
				require.ErrorContains(t, err, tc.expectError.Error())
			} else {
				require.NoError(t, err)

				actualClient, ok := ac.(*riakClient)
				require.True(t, ok)

				require.Equal(t, tc.cfg.Username, actualClient.creds.username)
				require.Equal(t, string(tc.cfg.Password), actualClient.creds.password)
				require.Equal(t, tc.cfg.Endpoint, actualClient.hostEndpoint)
				require.Equal(t, zap.NewNop(), actualClient.logger)
				require.NotNil(t, actualClient.client)
			}
		})
	}
}

func TestGetStatsDetails(t *testing.T) {
	t.Run("Non-200 Response", func(t *testing.T) {
		// Setup test server
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer ts.Close()

		tc := createTestClient(t, ts.URL)

		clusters, err := tc.GetStats(context.Background())
		require.Nil(t, clusters)
		require.EqualError(t, err, "non 200 code returned 401")
	})

	t.Run("Successful call", func(t *testing.T) {
		data := loadAPIResponseData(t, statsAPIResponseFile)

		// Setup test server
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write(data)
			assert.NoError(t, err)
		}))
		defer ts.Close()

		tc := createTestClient(t, ts.URL)

		// Load the valid data into a struct to compare
		var expected *model.Stats
		err := json.Unmarshal(data, &expected)
		require.NoError(t, err)

		clusters, err := tc.GetStats(context.Background())
		require.NoError(t, err)
		require.Equal(t, expected, clusters)
	})
}

func createTestClient(t *testing.T, baseEndpoint string) client {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = baseEndpoint

	testClient, err := newClient(context.Background(), cfg, componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings(), zap.NewNop())
	require.NoError(t, err)
	return testClient
}

func loadAPIResponseData(t *testing.T, fileName string) []byte {
	t.Helper()
	fullPath := filepath.Join("testdata", "apiresponses", fileName)

	data, err := os.ReadFile(fullPath)
	require.NoError(t, err)

	return data
}
