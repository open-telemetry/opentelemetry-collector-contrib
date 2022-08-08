// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bigipreceiver

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

func TestBigIpIntegration(t *testing.T) {
	mockServer := setupMockIControlServer(t)
	defer mockServer.Close()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	// Set endpoint of config to match mocked IControl REST API server
	cfg.Endpoint = mockServer.URL
	// Any username/password will work with mocked server right now
	cfg.Username = "otelu"
	cfg.Password = "otelp"
	cfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond

	consumer := new(consumertest.MetricsSink)
	settings := componenttest.NewNopReceiverCreateSettings()
	rcvr, err := factory.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)

	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.Eventuallyf(t, func() bool {
		return consumer.DataPointCount() > 0
	}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
	require.NoError(t, rcvr.Shutdown(context.Background()))

	actualMetrics := consumer.AllMetrics()[0]

	expectedFile := filepath.Join("testdata", "integration", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues()))
}

const (
	authHeader = "X-F5-Auth-Token"

	loginURISuffix                  = "/authn/login"
	getVirtualServersURISuffix      = "/ltm/virtual"
	getVirtualServersStatsURISuffix = "/ltm/virtual/stats"
	getPoolsStatsURISuffix          = "/ltm/pool/stats"
	getPoolMembersStatsURISuffix    = "/members/stats"
	getNodesStatsURISuffix          = "/ltm/node/stats"

	mockLoginResponseFile               = "login_response.json"
	mockVirtualServersResponseFile      = "virtual_servers_response.json"
	mockVirtualServersStatsResponseFile = "virtual_servers_stats_response.json"
	mockPoolsStatsResponseFile          = "pools_stats_response.json"
	mockNodesStatsResponseFile          = "nodes_stats_response.json"
	poolMembersStatsResponseFileSuffix  = "_pool_members_stats_response.json"
)

func setupMockIControlServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Setup responses before creating the server
	mockLoginResponse := createMockServerResponseData(t, mockLoginResponseFile)
	mockVirtualServersResponse := createMockServerResponseData(t, mockVirtualServersResponseFile)
	mockVirtualServersStatsResponse := createMockServerResponseData(t, mockVirtualServersStatsResponseFile)
	mockPoolsStatsResponse := createMockServerResponseData(t, mockPoolsStatsResponseFile)
	mockNodesStatsResponse := createMockServerResponseData(t, mockNodesStatsResponseFile)

	type loginBody struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// Setup mock iControl REST API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock login responses
		var err error
		if strings.HasSuffix(r.RequestURI, loginURISuffix) {
			var body loginBody
			err = json.NewDecoder(r.Body).Decode(&body)
			require.NoError(t, err)
			if body.Username == "" || body.Password == "" || r.Method != "POST" {
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				_, err = w.Write(mockLoginResponse)
				require.NoError(t, err)
			}

			return
		}

		// Mock response if no auth token header
		if r.Header.Get(authHeader) == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch {
		case strings.HasSuffix(r.RequestURI, getVirtualServersURISuffix):
			_, err = w.Write(mockVirtualServersResponse)
		case strings.HasSuffix(r.RequestURI, getVirtualServersStatsURISuffix):
			_, err = w.Write(mockVirtualServersStatsResponse)
		case strings.HasSuffix(r.RequestURI, getPoolsStatsURISuffix):
			_, err = w.Write(mockPoolsStatsResponse)
		case strings.HasSuffix(r.RequestURI, getNodesStatsURISuffix):
			_, err = w.Write(mockNodesStatsResponse)
		case strings.HasSuffix(r.RequestURI, getPoolMembersStatsURISuffix):
			// Assume pool member response files follow a specific file pattern based of pool name
			poolURI := strings.TrimSuffix(r.RequestURI, getPoolMembersStatsURISuffix)
			poolURIParts := strings.Split(poolURI, "/")
			poolName := strings.ReplaceAll(poolURIParts[len(poolURIParts)-1], "~", "_")
			poolMembersStatsData := createMockServerResponseData(t, poolName+poolMembersStatsResponseFileSuffix)
			_, err = w.Write(poolMembersStatsData)
		default:
			w.WriteHeader(http.StatusBadRequest)
			err = nil
		}
		require.NoError(t, err)
	}))

	return server
}

func createMockServerResponseData(t *testing.T, fileName string) []byte {
	t.Helper()
	fullPath := filepath.Join("testdata", "integration", "mock_server", fileName)

	data, err := ioutil.ReadFile(fullPath)
	require.NoError(t, err)

	return data
}
