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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
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
	step               func(*healthCheckExtension) error
	expectedStatusCode int
	expectedBody       string
}

func TestHealthCheckExtensionUsage(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		teststeps []teststep
	}{
		{
			name: "WithoutCheckCollectorPipeline",
			config: Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
				Path:                   "/",
				ResponseBody:           nil,
			},
			teststeps: []teststep{
				{
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       expectedBodyNotReady,
				},
				{
					step:               func(hcExt *healthCheckExtension) error { return hcExt.Ready() },
					expectedStatusCode: http.StatusOK,
					expectedBody:       expectedBodyReady,
				},
				{
					step:               func(hcExt *healthCheckExtension) error { return hcExt.NotReady() },
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       expectedBodyNotReady,
				},
			},
		},
		{
			name: "WithCustomizedPathWithoutCheckCollectorPipeline",
			config: Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
				Path:                   "/health",
			},
			teststeps: []teststep{
				{
					expectedStatusCode: http.StatusServiceUnavailable,
				},
				{
					step:               func(hcExt *healthCheckExtension) error { return hcExt.Ready() },
					expectedStatusCode: http.StatusOK,
				},
				{
					step:               func(hcExt *healthCheckExtension) error { return hcExt.NotReady() },
					expectedStatusCode: http.StatusServiceUnavailable,
				},
			},
		},
		{
			name: "WithBothCustomResponseBodyWithoutCheckCollectorPipeline",
			config: Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
				Path:                   "/",
				ResponseBody:           &ResponseBodySettings{Healthy: "ALL OK", Unhealthy: "NOT OK"},
			},
			teststeps: []teststep{
				{
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
				{
					step:               func(hcExt *healthCheckExtension) error { return hcExt.Ready() },
					expectedStatusCode: http.StatusOK,
					expectedBody:       "ALL OK",
				},
				{
					step:               func(hcExt *healthCheckExtension) error { return hcExt.NotReady() },
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
			},
		},
		{
			name: "WithHealthyCustomResponseBodyWithoutCheckCollectorPipeline",
			config: Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
				Path:                   "/",
				ResponseBody:           &ResponseBodySettings{Healthy: "ALL OK"},
			},
			teststeps: []teststep{
				{
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "",
				},
				{
					step:               func(hcExt *healthCheckExtension) error { return hcExt.Ready() },
					expectedStatusCode: http.StatusOK,
					expectedBody:       "ALL OK",
				},
				{
					step:               func(hcExt *healthCheckExtension) error { return hcExt.NotReady() },
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "",
				},
			},
		},
		{
			name: "WithUnhealthyCustomResponseBodyWithoutCheckCollectorPipeline",
			config: Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
				Path:                   "/",
				ResponseBody:           &ResponseBodySettings{Unhealthy: "NOT OK"},
			},
			teststeps: []teststep{
				{
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
				{
					step:               func(hcExt *healthCheckExtension) error { return hcExt.Ready() },
					expectedStatusCode: http.StatusOK,
					expectedBody:       "",
				},
				{
					step:               func(hcExt *healthCheckExtension) error { return hcExt.NotReady() },
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hcExt := newServer(tt.config, componenttest.NewNopTelemetrySettings())
			require.NotNil(t, hcExt)

			require.NoError(t, hcExt.Start(context.Background(), componenttest.NewNopHost()))
			t.Cleanup(func() { require.NoError(t, hcExt.Shutdown(context.Background())) })

			// Give a chance for the server goroutine to run.
			runtime.Gosched()
			require.Eventuallyf(t, ensureServerRunning(tt.config.Endpoint), 30*time.Second, 1*time.Second, "Failed to start the testing server.")

			client := &http.Client{}
			url := "http://" + tt.config.Endpoint + tt.config.Path

			for _, ts := range tt.teststeps {
				if ts.step != nil {
					require.NoError(t, ts.step(hcExt))
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

func TestHealthCheckExtensionPortAlreadyInUse(t *testing.T) {
	endpoint := testutil.GetAvailableLocalAddress(t)

	// This needs to be ":port" because health checks also tries to connect to ":port".
	// To avoid the pop-up "accept incoming network connections" health check should be changed
	// to accept an address.
	ln, err := net.Listen("tcp", endpoint)
	require.NoError(t, err)
	defer ln.Close()

	config := Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: endpoint,
		},
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
	}
	hcExt := newServer(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, hcExt)

	require.Error(t, hcExt.Start(context.Background(), componenttest.NewNopHost()))
}

func TestHealthCheckMultipleStarts(t *testing.T) {
	config := Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
		Path:                   "/",
	}

	hcExt := newServer(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, hcExt)

	require.NoError(t, hcExt.Start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, hcExt.Shutdown(context.Background())) })

	require.Error(t, hcExt.Start(context.Background(), componenttest.NewNopHost()))
}

func TestHealthCheckMultipleShutdowns(t *testing.T) {
	config := Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
		Path:                   "/",
	}

	hcExt := newServer(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, hcExt)

	require.NoError(t, hcExt.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, hcExt.Shutdown(context.Background()))
	require.NoError(t, hcExt.Shutdown(context.Background()))
}

func TestHealthCheckShutdownWithoutStart(t *testing.T) {
	config := Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		CheckCollectorPipeline: defaultCheckCollectorPipelineSettings(),
	}

	hcExt := newServer(config, componenttest.NewNopTelemetrySettings())
	require.NotNil(t, hcExt)

	require.NoError(t, hcExt.Shutdown(context.Background()))
}
