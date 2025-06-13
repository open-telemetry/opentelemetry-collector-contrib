// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/models"
)

const (
	queuesAPIResponseFile = "get_queues_response.json"
	nodesAPIResponseFile  = "get_nodes_response.json"
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
		host        component.Host
		settings    component.TelemetrySettings
		logger      *zap.Logger
		expectError error
	}{
		{
			desc: "Invalid HTTP config",
			cfg: &Config{
				ClientConfig: clientConfigNonexistentCA,
			},
			host:        componenttest.NewNopHost(),
			settings:    componenttest.NewNopTelemetrySettings(),
			logger:      zap.NewNop(),
			expectError: errors.New("failed to create HTTP Client"),
		},
		{
			desc: "Valid Configuration",
			cfg: &Config{
				ClientConfig: clientConfig,
			},
			host:        componenttest.NewNopHost(),
			settings:    componenttest.NewNopTelemetrySettings(),
			logger:      zap.NewNop(),
			expectError: nil,
		},
	}

	for _, tc := range testCase {
		t.Run(tc.desc, func(t *testing.T) {
			ac, err := newClient(context.Background(), tc.cfg, tc.host, tc.settings, tc.logger)
			if tc.expectError != nil {
				require.Nil(t, ac)
				require.ErrorContains(t, err, tc.expectError.Error())
			} else {
				require.NoError(t, err)

				actualClient, ok := ac.(*rabbitmqClient)
				require.True(t, ok)

				require.Equal(t, tc.cfg.Username, actualClient.creds.username)
				require.Equal(t, string(tc.cfg.Password), actualClient.creds.password)
				require.Equal(t, tc.cfg.Endpoint, actualClient.hostEndpoint)
				require.Equal(t, tc.logger, actualClient.logger)
				require.NotNil(t, actualClient.client)
			}
		})
	}
}

func TestGetQueuesDetails(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "Non-200 Response",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				clusters, err := tc.GetQueues(context.Background())
				require.Nil(t, clusters)
				require.EqualError(t, err, "non 200 code returned 401")
			},
		},
		{
			desc: "Bad payload returned",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, err := w.Write([]byte("{}"))
					assert.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				clusters, err := tc.GetQueues(context.Background())
				require.Nil(t, clusters)
				require.ErrorContains(t, err, "failed to decode response payload")
			},
		},
		{
			desc: "Successful call",
			testFunc: func(t *testing.T) {
				data := loadAPIResponseData(t, queuesAPIResponseFile)

				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, err := w.Write(data)
					assert.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				// Load the valid data into a struct to compare
				var expected []*models.Queue
				err := json.Unmarshal(data, &expected)
				require.NoError(t, err)

				clusters, err := tc.GetQueues(context.Background())
				require.NoError(t, err)
				require.Equal(t, expected, clusters)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func TestGetNodesDetails(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "Non-200 Response for GetNodes",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				nodes, err := tc.GetNodes(context.Background())
				require.Nil(t, nodes)
				require.EqualError(t, err, "non 200 code returned 403")
			},
		},
		{
			desc: "Bad payload returned for GetNodes",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, err := w.Write([]byte("{invalid-json}"))
					assert.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				nodes, err := tc.GetNodes(context.Background())
				require.Nil(t, nodes)
				require.ErrorContains(t, err, "failed to decode response payload")
			},
		},
		{
			desc: "Successful GetNodes call",
			testFunc: func(t *testing.T) {
				data := loadAPIResponseData(t, nodesAPIResponseFile)

				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, err := w.Write(data)
					assert.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				// Load the valid data into a struct to compare
				var expected []*models.Node
				err := json.Unmarshal(data, &expected)
				require.NoError(t, err)

				nodes, err := tc.GetNodes(context.Background())
				require.NoError(t, err)
				require.Equal(t, expected, nodes)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
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
