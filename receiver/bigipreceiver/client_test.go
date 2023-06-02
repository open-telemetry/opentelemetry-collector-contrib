// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bigipreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/models"
)

const (
	loginResponseFile               = "post_login_response.json"
	virtualServersResponseFile      = "get_virtual_servers_response.json"
	virtualServersStatsResponseFile = "get_virtual_servers_stats_response.json"
	virtualServersCombinedFile      = "virtual_servers_combined.json"
	poolsStatsResponseFile          = "get_pools_stats_response.json"
	poolMembersStatsResponse1File   = "get_pool_members_stats_response_1.json"
	poolMembersStatsResponse2File   = "get_pool_members_stats_response_2.json"
	poolMembersCombinedFile         = "pool_members_combined.json"
	nodesStatsResponseFile          = "get_nodes_stats_response.json"
)

func TestNewClient(t *testing.T) {
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
			logger:      zap.NewNop(),
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
			logger:      zap.NewNop(),
			expectError: nil,
		},
	}

	for _, tc := range testCase {
		t.Run(tc.desc, func(t *testing.T) {
			ac, err := newClient(tc.cfg, tc.host, tc.settings, tc.logger)
			if tc.expectError != nil {
				require.Nil(t, ac)
				require.Contains(t, err.Error(), tc.expectError.Error())
			} else {
				require.NoError(t, err)

				actualClient, ok := ac.(*bigipClient)
				require.True(t, ok)

				require.Equal(t, tc.cfg.Username, actualClient.creds.username)
				require.Equal(t, tc.cfg.Password, actualClient.creds.password)
				require.Equal(t, tc.cfg.Endpoint, actualClient.hostEndpoint)
				require.Equal(t, tc.logger, actualClient.logger)
				require.NotNil(t, actualClient.client)
			}
		})
	}
}

func TestGetNewToken(t *testing.T) {
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

				err := tc.GetNewToken(context.Background())
				require.EqualError(t, err, "non 200 code returned 401")
				hasToken := tc.HasToken()
				require.Equal(t, hasToken, false)
			},
		},
		{
			desc: "Bad payload returned",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := w.Write([]byte("[{}]"))
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				err := tc.GetNewToken(context.Background())
				require.Contains(t, err.Error(), "failed to decode response payload")
				hasToken := tc.HasToken()
				require.Equal(t, hasToken, false)
			},
		},
		{
			desc: "Successful call",
			testFunc: func(t *testing.T) {
				data := loadAPIResponseData(t, loginResponseFile)

				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := w.Write(data)
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				err := tc.GetNewToken(context.Background())
				require.NoError(t, err)
				hasToken := tc.HasToken()
				require.Equal(t, hasToken, true)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func TestGetVirtualServers(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "Non-200 Response for stats",
			testFunc: func(t *testing.T) {
				// Setup test server
				data := loadAPIResponseData(t, virtualServersResponseFile)
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.RequestURI, "stats") {
						w.WriteHeader(http.StatusUnauthorized)
					} else {
						_, err := w.Write(data)
						require.NoError(t, err)
					}
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				virtualServers, err := tc.GetVirtualServers(context.Background())
				require.Nil(t, virtualServers)
				require.EqualError(t, err, "non 200 code returned 401")
			},
		},
		{
			desc: "Bad payload returned for stats",
			testFunc: func(t *testing.T) {
				// Setup test server
				data := loadAPIResponseData(t, virtualServersResponseFile)
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var err error
					if strings.HasSuffix(r.RequestURI, "stats") {
						_, err = w.Write([]byte("[{}]"))
						require.NoError(t, err)
					} else {
						_, err = w.Write(data)
					}
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				pools, err := tc.GetPools(context.Background())
				require.Nil(t, pools)
				require.Contains(t, err.Error(), "failed to decode response payload")
			},
		},
		{
			desc: "Successful call empty body for stats",
			testFunc: func(t *testing.T) {
				// Setup test server
				data := loadAPIResponseData(t, virtualServersResponseFile)
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var err error
					if strings.HasSuffix(r.RequestURI, "stats") {
						_, err = w.Write([]byte("{}"))
					} else {
						_, err = w.Write(data)
					}
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				expected := models.VirtualServers{}
				virtualServers, err := tc.GetVirtualServers(context.Background())
				require.NoError(t, err)
				require.Equal(t, &expected, virtualServers)
			},
		},
		{
			desc: "Non-200 Response for standard",
			testFunc: func(t *testing.T) {
				// Setup test server
				statsData := loadAPIResponseData(t, virtualServersStatsResponseFile)
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasSuffix(r.RequestURI, "stats") {
						_, err := w.Write(statsData)
						require.NoError(t, err)
					} else {
						w.WriteHeader(http.StatusUnauthorized)
					}
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				var expected *models.VirtualServers
				err := json.Unmarshal(statsData, &expected)
				require.NoError(t, err)

				virtualServers, err := tc.GetVirtualServers(context.Background())
				require.NoError(t, err)
				require.Equal(t, expected, virtualServers)
			},
		},
		{
			desc: "Bad payload returned for standard",
			testFunc: func(t *testing.T) {
				// Setup test server
				statsData := loadAPIResponseData(t, virtualServersStatsResponseFile)
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var err error
					if strings.HasSuffix(r.RequestURI, "stats") {
						_, err = w.Write(statsData)
					} else {
						_, err = w.Write([]byte("[{}]"))
					}
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				var expected *models.VirtualServers
				err := json.Unmarshal(statsData, &expected)
				require.NoError(t, err)

				virtualServers, err := tc.GetVirtualServers(context.Background())
				require.NoError(t, err)
				require.Equal(t, expected, virtualServers)
			},
		},
		{
			desc: "Successful call empty body for stats",
			testFunc: func(t *testing.T) {
				// Setup test server
				statsData := loadAPIResponseData(t, virtualServersStatsResponseFile)
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var err error
					if strings.HasSuffix(r.RequestURI, "stats") {
						_, err = w.Write(statsData)
					} else {
						_, err = w.Write([]byte("{}"))
					}
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				var expected *models.VirtualServers
				err := json.Unmarshal(statsData, &expected)
				require.NoError(t, err)

				virtualServers, err := tc.GetVirtualServers(context.Background())
				require.NoError(t, err)
				require.Equal(t, expected, virtualServers)
			},
		},
		{
			desc: "Successful call",
			testFunc: func(t *testing.T) {
				data := loadAPIResponseData(t, virtualServersResponseFile)
				statsData := loadAPIResponseData(t, virtualServersStatsResponseFile)

				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var err error
					if strings.HasSuffix(r.RequestURI, "stats") {
						_, err = w.Write(statsData)
					} else {
						_, err = w.Write(data)
					}
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				// Load the valid data into a struct to compare
				var expected *models.VirtualServers
				combinedData := loadAPIResponseData(t, virtualServersCombinedFile)
				err := json.Unmarshal(combinedData, &expected)
				require.NoError(t, err)

				virtualServers, err := tc.GetVirtualServers(context.Background())
				require.NoError(t, err)
				require.Equal(t, expected, virtualServers)
			},
		},
		{
			desc: "Successful call empty body",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := w.Write([]byte("{}"))
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				expected := models.VirtualServers{}
				virtualServers, err := tc.GetVirtualServers(context.Background())
				require.NoError(t, err)
				require.Equal(t, &expected, virtualServers)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func TestGetPools(t *testing.T) {
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

				pools, err := tc.GetPools(context.Background())
				require.Nil(t, pools)
				require.EqualError(t, err, "non 200 code returned 401")
			},
		},
		{
			desc: "Bad payload returned",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := w.Write([]byte("[{}]"))
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				pools, err := tc.GetPools(context.Background())
				require.Nil(t, pools)
				require.Contains(t, err.Error(), "failed to decode response payload")
			},
		},
		{
			desc: "Successful call",
			testFunc: func(t *testing.T) {
				data := loadAPIResponseData(t, poolsStatsResponseFile)

				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := w.Write(data)
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				// Load the valid data into a struct to compare
				var expected *models.Pools
				err := json.Unmarshal(data, &expected)
				require.NoError(t, err)

				pools, err := tc.GetPools(context.Background())
				require.NoError(t, err)
				require.Equal(t, expected, pools)
			},
		},
		{
			desc: "Successful call empty body",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := w.Write([]byte("{}"))
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				expected := models.Pools{}
				pools, err := tc.GetPools(context.Background())
				require.NoError(t, err)
				require.Equal(t, &expected, pools)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func TestGetPoolMembers(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "Non-200 Response for all",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				var pools *models.Pools
				err := json.Unmarshal(loadAPIResponseData(t, poolsStatsResponseFile), &pools)
				require.NoError(t, err)

				poolMembers, err := tc.GetPoolMembers(context.Background(), pools)
				require.Nil(t, poolMembers)
				require.EqualError(t, err, "all pool member requests have failed")
			},
		},
		{
			desc: "Bad payload returned for all",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := w.Write([]byte("[{}]"))
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				var pools *models.Pools
				err := json.Unmarshal(loadAPIResponseData(t, poolsStatsResponseFile), &pools)
				require.NoError(t, err)

				poolMembers, err := tc.GetPoolMembers(context.Background(), pools)
				require.Nil(t, poolMembers)
				require.EqualError(t, err, "all pool member requests have failed")
			},
		},
		{
			desc: "Successful call for some",
			testFunc: func(t *testing.T) {
				data1 := loadAPIResponseData(t, poolMembersStatsResponse1File)

				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.RequestURI, "~Common~dev") {
						_, err := w.Write(data1)
						require.NoError(t, err)
					} else {
						w.WriteHeader(http.StatusUnauthorized)
					}
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				var pools *models.Pools
				err := json.Unmarshal(loadAPIResponseData(t, poolsStatsResponseFile), &pools)
				require.NoError(t, err)

				var expected *models.PoolMembers
				err = json.Unmarshal(data1, &expected)
				require.NoError(t, err)

				poolMembers, err := tc.GetPoolMembers(context.Background(), pools)
				require.EqualError(t, err, errors.New("non 200 code returned 401").Error())
				require.Equal(t, expected, poolMembers)
			},
		}, {
			desc: "Successful call empty body for some",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.RequestURI, "~Common~dev") {
						_, err := w.Write([]byte("{}"))
						require.NoError(t, err)
					} else {
						w.WriteHeader(http.StatusUnauthorized)
					}
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				var pools *models.Pools
				err := json.Unmarshal(loadAPIResponseData(t, poolsStatsResponseFile), &pools)
				require.NoError(t, err)

				expected := models.PoolMembers{}

				poolMembers, err := tc.GetPoolMembers(context.Background(), pools)
				require.EqualError(t, err, errors.New("non 200 code returned 401").Error())
				require.Equal(t, &expected, poolMembers)
			},
		},
		{
			desc: "Successful call for all",
			testFunc: func(t *testing.T) {
				data1 := loadAPIResponseData(t, poolMembersStatsResponse1File)
				data2 := loadAPIResponseData(t, poolMembersStatsResponse2File)

				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var err error
					if strings.Contains(r.RequestURI, "~Common~dev") {
						_, err = w.Write(data1)
					} else {
						_, err = w.Write(data2)
					}
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				var pools *models.Pools
				err := json.Unmarshal(loadAPIResponseData(t, poolsStatsResponseFile), &pools)
				require.NoError(t, err)

				var expected *models.PoolMembers
				combinedData := loadAPIResponseData(t, poolMembersCombinedFile)
				err = json.Unmarshal(combinedData, &expected)
				require.NoError(t, err)

				poolMembers, err := tc.GetPoolMembers(context.Background(), pools)
				require.NoError(t, err)
				require.Equal(t, expected, poolMembers)
			},
		},
		{
			desc: "Successful call empty body for all",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := w.Write([]byte("{}"))
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				var pools *models.Pools
				err := json.Unmarshal(loadAPIResponseData(t, poolsStatsResponseFile), &pools)
				require.NoError(t, err)
				expected := models.PoolMembers{}

				poolMembers, err := tc.GetPoolMembers(context.Background(), pools)
				require.NoError(t, err)
				require.Equal(t, &expected, poolMembers)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func TestGetNodes(t *testing.T) {
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

				nodes, err := tc.GetNodes(context.Background())
				require.Nil(t, nodes)
				require.EqualError(t, err, "non 200 code returned 401")
			},
		},
		{
			desc: "Bad payload returned",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := w.Write([]byte("[{}]"))
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				nodes, err := tc.GetNodes(context.Background())
				require.Nil(t, nodes)
				require.Contains(t, err.Error(), "failed to decode response payload")
			},
		},
		{
			desc: "Successful call",
			testFunc: func(t *testing.T) {
				data := loadAPIResponseData(t, nodesStatsResponseFile)

				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := w.Write(data)
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				// Load the valid data into a struct to compare
				var expected *models.Nodes
				err := json.Unmarshal(data, &expected)
				require.NoError(t, err)

				nodes, err := tc.GetNodes(context.Background())
				require.NoError(t, err)
				require.Equal(t, expected, nodes)
			},
		},
		{
			desc: "Successful call empty body",
			testFunc: func(t *testing.T) {
				// Setup test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := w.Write([]byte("{}"))
					require.NoError(t, err)
				}))
				defer ts.Close()

				tc := createTestClient(t, ts.URL)

				expected := models.Nodes{}
				nodes, err := tc.GetNodes(context.Background())
				require.NoError(t, err)
				require.Equal(t, &expected, nodes)
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

	testClient, err := newClient(cfg, componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings(), zap.NewNop())
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
