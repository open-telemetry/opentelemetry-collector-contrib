// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension

import (
	"context"
	"io"
	"net"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

const (
	expectedBodyNotReady = "{\"status\":\"Server not available\",\"upSince\":"
	expectedBodyReady    = "{\"status\":\"Server available\",\"upSince\":"
)

func ensureServerRunning(url string) func() bool {
	return func() bool {
		_, err := net.DialTimeout("tcp", url, 30*time.Second)
		return err == nil
	}
}

type teststep struct {
	step               func(extensioncapabilities.PipelineWatcher) error
	expectedStatusCode int
	expectedBody       string
}

func TestHealthCheckExtensionUsage(t *testing.T) {
	serverConfigWithoutCheckCollectorPipeline := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfigWithoutCheckCollectorPipeline.WriteTimeout = 0
	serverConfigWithoutCheckCollectorPipeline.ReadHeaderTimeout = 0
	serverConfigWithoutCheckCollectorPipeline.IdleTimeout = 0
	serverConfigWithoutCheckCollectorPipeline.KeepAlivesEnabled = false
	serverConfigWithoutCheckCollectorPipeline.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfigWithCustomizedPath := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfigWithCustomizedPath.WriteTimeout = 0
	serverConfigWithCustomizedPath.ReadHeaderTimeout = 0
	serverConfigWithCustomizedPath.IdleTimeout = 0
	serverConfigWithCustomizedPath.KeepAlivesEnabled = false
	serverConfigWithCustomizedPath.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfigWithBothCustomResponseBody := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfigWithBothCustomResponseBody.WriteTimeout = 0
	serverConfigWithBothCustomResponseBody.ReadHeaderTimeout = 0
	serverConfigWithBothCustomResponseBody.IdleTimeout = 0
	serverConfigWithBothCustomResponseBody.KeepAlivesEnabled = false
	serverConfigWithBothCustomResponseBody.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfigWithHealthyCustomResponseBody := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfigWithHealthyCustomResponseBody.WriteTimeout = 0
	serverConfigWithHealthyCustomResponseBody.ReadHeaderTimeout = 0
	serverConfigWithHealthyCustomResponseBody.IdleTimeout = 0
	serverConfigWithHealthyCustomResponseBody.KeepAlivesEnabled = false
	serverConfigWithHealthyCustomResponseBody.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfigWithUnhealthyCustomResponseBody := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfigWithUnhealthyCustomResponseBody.WriteTimeout = 0
	serverConfigWithUnhealthyCustomResponseBody.ReadHeaderTimeout = 0
	serverConfigWithUnhealthyCustomResponseBody.IdleTimeout = 0
	serverConfigWithUnhealthyCustomResponseBody.KeepAlivesEnabled = false
	serverConfigWithUnhealthyCustomResponseBody.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	tests := []struct {
		name      string
		config    *Config
		teststeps []teststep
	}{
		{
			name: "WithoutCheckCollectorPipeline",
			config: &Config{
				Config: healthcheck.Config{
					LegacyConfig: healthcheck.HTTPLegacyConfig{
						ServerConfig: serverConfigWithoutCheckCollectorPipeline,
						Path:         "/",
						CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
							Enabled:                  false,
							Interval:                 "5m",
							ExporterFailureThreshold: 5,
						},
					},
				},
			},
			teststeps: []teststep{
				{
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       expectedBodyNotReady,
				},
				{
					step:               func(hcExt extensioncapabilities.PipelineWatcher) error { return hcExt.Ready() },
					expectedStatusCode: http.StatusOK,
					expectedBody:       expectedBodyReady,
				},
				{
					step:               func(hcExt extensioncapabilities.PipelineWatcher) error { return hcExt.NotReady() },
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       expectedBodyNotReady,
				},
			},
		},
		{
			name: "WithCustomizedPathWithoutCheckCollectorPipeline",
			config: &Config{
				Config: healthcheck.Config{
					LegacyConfig: healthcheck.HTTPLegacyConfig{
						ServerConfig: serverConfigWithCustomizedPath,
						Path:         "/health",
						CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
							Enabled:                  false,
							Interval:                 "5m",
							ExporterFailureThreshold: 5,
						},
					},
				},
			},
			teststeps: []teststep{
				{
					expectedStatusCode: http.StatusServiceUnavailable,
				},
				{
					step:               func(hcExt extensioncapabilities.PipelineWatcher) error { return hcExt.Ready() },
					expectedStatusCode: http.StatusOK,
				},
				{
					step:               func(hcExt extensioncapabilities.PipelineWatcher) error { return hcExt.NotReady() },
					expectedStatusCode: http.StatusServiceUnavailable,
				},
			},
		},
		{
			name: "WithBothCustomResponseBodyWithoutCheckCollectorPipeline",
			config: &Config{
				Config: healthcheck.Config{
					LegacyConfig: healthcheck.HTTPLegacyConfig{
						ServerConfig: serverConfigWithBothCustomResponseBody,
						Path:         "/",
						ResponseBody: &healthcheck.ResponseBodyConfig{
							Healthy:   "ALL OK",
							Unhealthy: "NOT OK",
						},
						CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
							Enabled:                  false,
							Interval:                 "5m",
							ExporterFailureThreshold: 5,
						},
					},
				},
			},
			teststeps: []teststep{
				{
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
				{
					step:               func(hcExt extensioncapabilities.PipelineWatcher) error { return hcExt.Ready() },
					expectedStatusCode: http.StatusOK,
					expectedBody:       "ALL OK",
				},
				{
					step:               func(hcExt extensioncapabilities.PipelineWatcher) error { return hcExt.NotReady() },
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
			},
		},
		{
			name: "WithHealthyCustomResponseBodyWithoutCheckCollectorPipeline",
			config: &Config{
				Config: healthcheck.Config{
					LegacyConfig: healthcheck.HTTPLegacyConfig{
						ServerConfig: serverConfigWithHealthyCustomResponseBody,
						Path:         "/",
						ResponseBody: &healthcheck.ResponseBodyConfig{
							Healthy: "ALL OK",
						},
						CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
							Enabled:                  false,
							Interval:                 "5m",
							ExporterFailureThreshold: 5,
						},
					},
				},
			},
			teststeps: []teststep{
				{
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "",
				},
				{
					step:               func(hcExt extensioncapabilities.PipelineWatcher) error { return hcExt.Ready() },
					expectedStatusCode: http.StatusOK,
					expectedBody:       "ALL OK",
				},
				{
					step:               func(hcExt extensioncapabilities.PipelineWatcher) error { return hcExt.NotReady() },
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "",
				},
			},
		},
		{
			name: "WithUnhealthyCustomResponseBodyWithoutCheckCollectorPipeline",
			config: &Config{
				Config: healthcheck.Config{
					LegacyConfig: healthcheck.HTTPLegacyConfig{
						ServerConfig: serverConfigWithUnhealthyCustomResponseBody,
						Path:         "/",
						ResponseBody: &healthcheck.ResponseBodyConfig{
							Unhealthy: "NOT OK",
						},
						CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
							Enabled:                  false,
							Interval:                 "5m",
							ExporterFailureThreshold: 5,
						},
					},
				},
			},
			teststeps: []teststep{
				{
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
				{
					step:               func(hcExt extensioncapabilities.PipelineWatcher) error { return hcExt.Ready() },
					expectedStatusCode: http.StatusOK,
					expectedBody:       "",
				},
				{
					step:               func(hcExt extensioncapabilities.PipelineWatcher) error { return hcExt.NotReady() },
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hcExt, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), tt.config)
			require.NoError(t, err)
			require.NotNil(t, hcExt)

			require.NoError(t, hcExt.Start(t.Context(), componenttest.NewNopHost()))
			t.Cleanup(func() {
				// cleanup functions run after test context is cancelled
				require.NoError(t, hcExt.Shutdown(context.WithoutCancel(t.Context())))
			})

			// Give a chance for the server goroutine to run.
			runtime.Gosched()
			require.Eventuallyf(t, ensureServerRunning(tt.config.NetAddr.Endpoint), 30*time.Second, 1*time.Second, "Failed to start the testing server.")

			client := &http.Client{}
			url := "http://" + tt.config.NetAddr.Endpoint + tt.config.Path

			// Cast to PipelineWatcher for step functions
			pw, ok := hcExt.(extensioncapabilities.PipelineWatcher)
			require.True(t, ok, "extension must implement PipelineWatcher")

			for _, ts := range tt.teststeps {
				if ts.step != nil {
					require.NoError(t, ts.step(pw))
				}

				resp, err := client.Get(url)
				require.NoError(t, err)

				if ts.expectedStatusCode != 0 {
					require.Equal(t, ts.expectedStatusCode, resp.StatusCode)
				}
				if ts.expectedBody != "" {
					body, err := io.ReadAll(resp.Body)
					require.NoError(t, err)
					require.Contains(t, string(body), ts.expectedBody)
				}
				require.NoError(t, resp.Body.Close(), "Must be able to close the response")
			}
		})
	}
}

func TestHealthCheckShutdownWithoutStart(t *testing.T) {
	serverConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig.WriteTimeout = 0
	serverConfig.ReadHeaderTimeout = 0
	serverConfig.IdleTimeout = 0
	serverConfig.KeepAlivesEnabled = false
	serverConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	config := &Config{
		Config: healthcheck.Config{
			LegacyConfig: healthcheck.HTTPLegacyConfig{
				ServerConfig: serverConfig,
				Path:         "/",
				CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
					Enabled:                  false,
					Interval:                 "5m",
					ExporterFailureThreshold: 5,
				},
			},
		},
	}

	hcExt, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), config)
	require.NoError(t, err)
	require.NotNil(t, hcExt)

	require.NoError(t, hcExt.Shutdown(t.Context()))
}
