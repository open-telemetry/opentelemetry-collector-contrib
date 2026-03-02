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
						ServerConfig: confighttp.ServerConfig{
							NetAddr: confignet.AddrConfig{
								Transport: "tcp",
								Endpoint:  testutil.GetAvailableLocalAddress(t),
							},
						},
						Path: "/",
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
						ServerConfig: confighttp.ServerConfig{
							NetAddr: confignet.AddrConfig{
								Transport: "tcp",
								Endpoint:  testutil.GetAvailableLocalAddress(t),
							},
						},
						Path: "/health",
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
						ServerConfig: confighttp.ServerConfig{
							NetAddr: confignet.AddrConfig{
								Transport: "tcp",
								Endpoint:  testutil.GetAvailableLocalAddress(t),
							},
						},
						Path: "/",
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
						ServerConfig: confighttp.ServerConfig{
							NetAddr: confignet.AddrConfig{
								Transport: "tcp",
								Endpoint:  testutil.GetAvailableLocalAddress(t),
							},
						},
						Path: "/",
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
						ServerConfig: confighttp.ServerConfig{
							NetAddr: confignet.AddrConfig{
								Transport: "tcp",
								Endpoint:  testutil.GetAvailableLocalAddress(t),
							},
						},
						Path: "/",
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
			// Use context.Background() instead of t.Context() because createExtension starts
			// a background goroutine that needs to stay alive for the duration of the test.
			// t.Context() may be cancelled immediately causing the goroutine to exit prematurely.
			//nolint:usetesting
			hcExt, err := createExtension(context.Background(), extensiontest.NewNopSettings(extensiontest.NopType), tt.config)
			require.NoError(t, err)
			require.NotNil(t, hcExt)

			//nolint:usetesting
			require.NoError(t, hcExt.Start(context.Background(), componenttest.NewNopHost()))
			//nolint:usetesting
			t.Cleanup(func() { require.NoError(t, hcExt.Shutdown(context.Background())) })

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
	config := &Config{
		Config: healthcheck.Config{
			LegacyConfig: healthcheck.HTTPLegacyConfig{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Transport: "tcp",
						Endpoint:  testutil.GetAvailableLocalAddress(t),
					},
				},
				Path: "/",
				CheckCollectorPipeline: &healthcheck.CheckCollectorPipelineConfig{
					Enabled:                  false,
					Interval:                 "5m",
					ExporterFailureThreshold: 5,
				},
			},
		},
	}

	// Use context.Background() instead of t.Context() because createExtension starts
	// a background goroutine that needs to stay alive for the duration of the test.
	// t.Context() may be cancelled immediately causing the goroutine to exit prematurely.
	//nolint:usetesting
	hcExt, err := createExtension(context.Background(), extensiontest.NewNopSettings(extensiontest.NopType), config)
	require.NoError(t, err)
	require.NotNil(t, hcExt)

	//nolint:usetesting
	require.NoError(t, hcExt.Shutdown(context.Background()))
}
