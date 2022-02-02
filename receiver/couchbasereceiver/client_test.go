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

package couchbasereceiver

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
)

const (
	clusterAPIResponseFile       = "get_clusters_response.json"
	clusterBucketAPIResponseFile = "get_cluster_bucket_info_response.json"
	bucketStatsAPIResponseFile   = "get_bucket_stats.json"
)

func TestNewClient(t *testing.T) {
	testCase := []struct {
		desc        string
		cfg         *Config
		host        component.Host
		settings    component.TelemetrySettings
		expectError error
	}{
		{
			desc: "Invalid HTTP config",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: defaultEndpoint,
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CAFile: "/non/existent",
						},
					},
				},
			},
			host:        componenttest.NewNopHost(),
			settings:    componenttest.NewNopTelemetrySettings(),
			expectError: errors.New("failed to create HTTP Client"),
		},
		{
			desc: "Valid Configuration",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					TLSSetting: configtls.TLSClientSetting{},
					Endpoint:   defaultEndpoint,
				},
			},
			host:        componenttest.NewNopHost(),
			settings:    componenttest.NewNopTelemetrySettings(),
			expectError: nil,
		},
	}

	for _, tc := range testCase {
		t.Run(tc.desc, func(t *testing.T) {
			ac, err := newClient(tc.cfg, tc.host, tc.settings)
			if tc.expectError != nil {
				require.Nil(t, ac)
				require.Contains(t, err.Error(), tc.expectError.Error())
			} else {
				require.NoError(t, err)

				actualClient, ok := ac.(*couchbaseClient)
				require.True(t, ok)

				require.Equal(t, tc.cfg.Username, actualClient.creds.username)
				require.Equal(t, tc.cfg.Password, actualClient.creds.password)
				require.Equal(t, tc.cfg.Endpoint, actualClient.hostEndpoint)
				require.Equal(t, tc.settings.Logger, actualClient.logger)
				require.NotNil(t, actualClient.client)
			}
		})
	}
}

func TestGetClusterDetails(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "Non-200 Response",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				clusters, err := tc.GetClusterDetails(context.Background())
				require.Nil(t, clusters)
				require.EqualError(t, err, "non 200 code returned 401")
			},
		},
		{
			desc: "Bad payload returned",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("[]"))
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				clusters, err := tc.GetClusterDetails(context.Background())
				require.Nil(t, clusters)
				require.Contains(t, err.Error(), "failed to decode response payload")
			},
		},
		{
			desc: "Successful call",
			testFunc: func(t *testing.T) {
				data := loadAPIResponseData(t, clusterAPIResponseFile)

				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write(data)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				// Load the valid data into a struct to compare
				var expected clusterResponse
				err := json.Unmarshal(data, &expected)
				require.NoError(t, err)

				clusters, err := tc.GetClusterDetails(context.Background())
				require.NoError(t, err)
				require.Equal(t, &expected, clusters)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func TestGetBuckets(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "Non-200 Response",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				buckets, err := tc.GetBuckets(context.Background(), "/path/to/bucket_info")
				require.Nil(t, buckets)
				require.EqualError(t, err, "non 200 code returned 401")
			},
		},
		{
			desc: "Bad payload returned",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("{}"))
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				buckets, err := tc.GetBuckets(context.Background(), "/path/to/bucket_info")
				require.Nil(t, buckets)
				require.Contains(t, err.Error(), "failed to decode response payload")
			},
		},
		{
			desc: "Successful call",
			testFunc: func(t *testing.T) {
				data := loadAPIResponseData(t, clusterBucketAPIResponseFile)

				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write(data)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				// Load the valid data into a struct to compare
				var expected []*bucket
				err := json.Unmarshal(data, &expected)
				require.NoError(t, err)

				buckets, err := tc.GetBuckets(context.Background(), "/path/to/bucket_info")
				require.NoError(t, err)
				require.Equal(t, expected, buckets)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func TestGetBucketStats(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "Non-200 Response",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				stats, err := tc.GetBucketStats(context.Background(), "/path/to/stats")
				require.Nil(t, stats)
				require.EqualError(t, err, "non 200 code returned 401")
			},
		},
		{
			desc: "Bad payload returned",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("[]"))
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				stats, err := tc.GetBucketStats(context.Background(), "/path/to/stats")
				require.Nil(t, stats)
				require.Contains(t, err.Error(), "failed to decode response payload")
			},
		},
		{
			desc: "Successful call",
			testFunc: func(t *testing.T) {
				data := loadAPIResponseData(t, bucketStatsAPIResponseFile)

				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write(data)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				// Load the valid data into a struct to compare
				var expected bucketStats
				err := json.Unmarshal(data, &expected)
				require.NoError(t, err)

				stats, err := tc.GetBucketStats(context.Background(), "/path/to/stats")
				require.NoError(t, err)
				require.Equal(t, &expected, stats)
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

	testClient, err := newClient(cfg, componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	return testClient
}

func loadAPIResponseData(t *testing.T, fileName string) []byte {
	t.Helper()
	fullPath := filepath.Join("testdata", "apiresponses", fileName)

	data, err := ioutil.ReadFile(fullPath)
	require.NoError(t, err)

	return data
}
