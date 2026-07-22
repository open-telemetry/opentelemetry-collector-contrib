// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/goleak"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	hcconfig "github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/config"
	internalhelpers "github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck/internal/testhelpers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status/testhelpers"
)

// These are used for the legacy test assertions
const (
	expectedBodyNotReady = "{\"status\":\"Server not available\",\"upSince\":"
	expectedBodyReady    = "{\"status\":\"Server available\",\"upSince\":"
)

var (
	componentStatusOK = &componentStatusExpectation{
		healthy: true,
		status:  componentstatus.StatusOK,
	}
	componentStatusPipelineMetricsStarting = map[string]*componentStatusExpectation{
		"receiver:metrics/in": {
			healthy: true,
			status:  componentstatus.StatusStarting,
		},
		"processor:batch": {
			healthy: true,
			status:  componentstatus.StatusStarting,
		},
		"exporter:metrics/out": {
			healthy: true,
			status:  componentstatus.StatusStarting,
		},
	}
	componentStatusPipelineMetricsOK = map[string]*componentStatusExpectation{
		"receiver:metrics/in":  componentStatusOK,
		"processor:batch":      componentStatusOK,
		"exporter:metrics/out": componentStatusOK,
	}
	componentStatusPipelineMetricsStopping = map[string]*componentStatusExpectation{
		"receiver:metrics/in": {
			healthy: true,
			status:  componentstatus.StatusStopping,
		},
		"processor:batch": {
			healthy: true,
			status:  componentstatus.StatusStopping,
		},
		"exporter:metrics/out": {
			healthy: true,
			status:  componentstatus.StatusStopping,
		},
	}
	componentStatusPipelineMetricsStopped = map[string]*componentStatusExpectation{
		"receiver:metrics/in": {
			healthy: true,
			status:  componentstatus.StatusStopped,
		},
		"processor:batch": {
			healthy: true,
			status:  componentstatus.StatusStopped,
		},
		"exporter:metrics/out": {
			healthy: true,
			status:  componentstatus.StatusStopped,
		},
	}
	componentStatusPipelineTracesStarting = map[string]*componentStatusExpectation{
		"receiver:traces/in": {
			healthy: true,
			status:  componentstatus.StatusStarting,
		},
		"processor:batch": {
			healthy: true,
			status:  componentstatus.StatusStarting,
		},
		"exporter:traces/out": {
			healthy: true,
			status:  componentstatus.StatusStarting,
		},
	}
	componentStatusPipelineTracesOK = map[string]*componentStatusExpectation{
		"receiver:traces/in":  componentStatusOK,
		"processor:batch":     componentStatusOK,
		"exporter:traces/out": componentStatusOK,
	}
	componentStatusPipelineTracesStopping = map[string]*componentStatusExpectation{
		"receiver:traces/in": {
			healthy: true,
			status:  componentstatus.StatusStopping,
		},
		"processor:batch": {
			healthy: true,
			status:  componentstatus.StatusStopping,
		},
		"exporter:traces/out": {
			healthy: true,
			status:  componentstatus.StatusStopping,
		},
	}
	componentStatusPipelineTracesStopped = map[string]*componentStatusExpectation{
		"receiver:traces/in": {
			healthy: true,
			status:  componentstatus.StatusStopped,
		},
		"processor:batch": {
			healthy: true,
			status:  componentstatus.StatusStopped,
		},
		"exporter:traces/out": {
			healthy: true,
			status:  componentstatus.StatusStopped,
		},
	}
)

type componentStatusExpectation struct {
	healthy      bool
	status       componentstatus.Status
	err          error
	nestedStatus map[string]*componentStatusExpectation
}

type teststep struct {
	step                    func()
	queryParams             string
	eventually              bool
	expectedStatusCode      int
	expectedBody            string
	expectedComponentStatus *componentStatusExpectation
}

func TestStatus(t *testing.T) {
	// These goroutines are part of the http.Client's connection pool management.
	// They don't accept context.Context and are managed by the transport's lifecycle,
	// not our test lifecycle. They'll be cleaned up when the transport is garbage collected.
	opts := []goleak.Option{
		goleak.IgnoreCurrent(),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	}
	goleak.VerifyNone(t, opts...)

	var server *Server
	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)
	metrics := testhelpers.NewPipelineMetadata(pipeline.SignalMetrics)

	serverConfig1 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig1.WriteTimeout = 0
	serverConfig1.ReadHeaderTimeout = 0
	serverConfig1.IdleTimeout = 0
	serverConfig1.KeepAlivesEnabled = false
	serverConfig1.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig2 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig2.WriteTimeout = 0
	serverConfig2.ReadHeaderTimeout = 0
	serverConfig2.IdleTimeout = 0
	serverConfig2.KeepAlivesEnabled = false
	serverConfig2.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig3 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig3.WriteTimeout = 0
	serverConfig3.ReadHeaderTimeout = 0
	serverConfig3.IdleTimeout = 0
	serverConfig3.KeepAlivesEnabled = false
	serverConfig3.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig4 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig4.WriteTimeout = 0
	serverConfig4.ReadHeaderTimeout = 0
	serverConfig4.IdleTimeout = 0
	serverConfig4.KeepAlivesEnabled = false
	serverConfig4.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig5 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig5.WriteTimeout = 0
	serverConfig5.ReadHeaderTimeout = 0
	serverConfig5.IdleTimeout = 0
	serverConfig5.KeepAlivesEnabled = false
	serverConfig5.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig6 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig6.WriteTimeout = 0
	serverConfig6.ReadHeaderTimeout = 0
	serverConfig6.IdleTimeout = 0
	serverConfig6.KeepAlivesEnabled = false
	serverConfig6.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig7 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig7.WriteTimeout = 0
	serverConfig7.ReadHeaderTimeout = 0
	serverConfig7.IdleTimeout = 0
	serverConfig7.KeepAlivesEnabled = false
	serverConfig7.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig8 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig8.WriteTimeout = 0
	serverConfig8.ReadHeaderTimeout = 0
	serverConfig8.IdleTimeout = 0
	serverConfig8.KeepAlivesEnabled = false
	serverConfig8.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig9 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig9.WriteTimeout = 0
	serverConfig9.ReadHeaderTimeout = 0
	serverConfig9.IdleTimeout = 0
	serverConfig9.KeepAlivesEnabled = false
	serverConfig9.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig10 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig10.WriteTimeout = 0
	serverConfig10.ReadHeaderTimeout = 0
	serverConfig10.IdleTimeout = 0
	serverConfig10.KeepAlivesEnabled = false
	serverConfig10.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig11 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig11.WriteTimeout = 0
	serverConfig11.ReadHeaderTimeout = 0
	serverConfig11.IdleTimeout = 0
	serverConfig11.KeepAlivesEnabled = false
	serverConfig11.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig12 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig12.WriteTimeout = 0
	serverConfig12.ReadHeaderTimeout = 0
	serverConfig12.IdleTimeout = 0
	serverConfig12.KeepAlivesEnabled = false
	serverConfig12.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig13 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig13.WriteTimeout = 0
	serverConfig13.ReadHeaderTimeout = 0
	serverConfig13.IdleTimeout = 0
	serverConfig13.KeepAlivesEnabled = false
	serverConfig13.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	serverConfig14 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig14.WriteTimeout = 0
	serverConfig14.ReadHeaderTimeout = 0
	serverConfig14.IdleTimeout = 0
	serverConfig14.KeepAlivesEnabled = false
	serverConfig14.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}

	tests := []struct {
		name                  string
		config                *Config
		legacyConfig          LegacyConfig
		componentHealthConfig *hcconfig.ComponentHealthConfig
		teststeps             []teststep
	}{
		{
			name:         "exclude recoverable and permanent errors",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: serverConfig1,
				Config:       PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
					},
				},
				{
					queryParams:             "pipeline=traces",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					queryParams:             "pipeline=metrics",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:             "pipeline=traces",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewEvent(componentstatus.StatusOK),
						)
					},
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					queryParams:             "pipeline=metrics",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:             "pipeline=traces",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
					},
				},
				{
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
					},
				},
				{
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
					},
				},
			},
		},
		{
			name:         "exclude recoverable and permanent errors - verbose",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: serverConfig2,
				Config:       PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusStarting,
								nestedStatus: componentStatusPipelineTracesStarting,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStarting,
								nestedStatus: componentStatusPipelineMetricsStarting,
							},
						},
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStarting,
								nestedStatus: componentStatusPipelineMetricsStarting,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineTracesOK,
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:metrics/in": {
								healthy: true,
								status:  componentstatus.StatusStarting,
							},
							"processor:batch": {
								healthy: true,
								status:  componentstatus.StatusStarting,
							},
							"exporter:metrics/out": {
								healthy: true,
								status:  componentstatus.StatusStarting,
							},
						},
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineMetricsOK,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineMetricsOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy: true,
								status:  componentstatus.StatusRecoverableError,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:metrics/in": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"exporter:metrics/out": {
										healthy: true,
										status:  componentstatus.StatusRecoverableError,
										err:     assert.AnError,
									},
								},
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:metrics/in": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"exporter:metrics/out": {
								healthy: true,
								status:  componentstatus.StatusRecoverableError,
								err:     assert.AnError,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineTracesOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewEvent(componentstatus.StatusOK),
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineMetricsOK,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineMetricsOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy: true,
								status:  componentstatus.StatusPermanentError,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:metrics/in": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"exporter:metrics/out": {
										healthy: true,
										status:  componentstatus.StatusPermanentError,
										err:     assert.AnError,
									},
								},
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:metrics/in": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"exporter:metrics/out": {
								healthy: true,
								status:  componentstatus.StatusPermanentError,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineTracesOK,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusStopping,
								nestedStatus: componentStatusPipelineTracesStopping,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStopping,
								nestedStatus: componentStatusPipelineMetricsStopping,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopping,
						nestedStatus: componentStatusPipelineTracesStopping,
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopping,
						nestedStatus: componentStatusPipelineMetricsStopping,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusStopped,
								nestedStatus: componentStatusPipelineTracesStopped,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStopped,
								nestedStatus: componentStatusPipelineMetricsStopped,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopped,
						nestedStatus: componentStatusPipelineTracesStopped,
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopped,
						nestedStatus: componentStatusPipelineMetricsStopped,
					},
				},
			},
		},
		{
			name:         "include recoverable and exclude permanent errors",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: serverConfig3,
				Config:       PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			componentHealthConfig: &hcconfig.ComponentHealthConfig{
				IncludePermanent:   false,
				IncludeRecoverable: true,
				RecoveryDuration:   2 * time.Millisecond,
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
					},
				},
				{
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					queryParams:             "pipeline=traces",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					queryParams:             "pipeline=metrics",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					eventually:         true,
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:             "pipeline=traces",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewEvent(componentstatus.StatusOK),
						)
					},
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					queryParams:             "pipeline=metrics",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:             "pipeline=traces",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
					},
				},
				{
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
					},
				},
				{
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
					},
				},
			},
		},
		{
			name:         "include recoverable and exclude permanent errors - verbose",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: serverConfig4,
				Config:       PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			componentHealthConfig: &hcconfig.ComponentHealthConfig{
				IncludePermanent:   false,
				IncludeRecoverable: true,
				RecoveryDuration:   2 * time.Millisecond,
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusStarting,
								nestedStatus: componentStatusPipelineTracesStarting,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStarting,
								nestedStatus: componentStatusPipelineMetricsStarting,
							},
						},
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineMetricsOK,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineMetricsOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					eventually:         true,
					queryParams:        "verbose",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy: false,
								status:  componentstatus.StatusRecoverableError,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:metrics/in": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"exporter:metrics/out": {
										healthy: false,
										status:  componentstatus.StatusRecoverableError,
										err:     assert.AnError,
									},
								},
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:metrics/in": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"exporter:metrics/out": {
								healthy: false,
								status:  componentstatus.StatusRecoverableError,
								err:     assert.AnError,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineTracesOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewEvent(componentstatus.StatusOK),
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineMetricsOK,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineMetricsOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy: true,
								status:  componentstatus.StatusPermanentError,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:metrics/in": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"exporter:metrics/out": {
										healthy: true,
										status:  componentstatus.StatusPermanentError,
										err:     assert.AnError,
									},
								},
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:metrics/in": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"exporter:metrics/out": {
								healthy: true,
								status:  componentstatus.StatusPermanentError,
							},
						},
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusStopping,
								nestedStatus: componentStatusPipelineTracesStopping,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStopping,
								nestedStatus: componentStatusPipelineMetricsStopping,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopping,
						nestedStatus: componentStatusPipelineTracesStopping,
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopping,
						nestedStatus: componentStatusPipelineMetricsStopping,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
						)
					},
					queryParams:        "",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusStopped,
								nestedStatus: componentStatusPipelineTracesStopped,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStopped,
								nestedStatus: componentStatusPipelineMetricsStopped,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopped,
						nestedStatus: componentStatusPipelineTracesStopped,
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopped,
						nestedStatus: componentStatusPipelineMetricsStopped,
					},
				},
			},
		},
		{
			name:         "include permanent and exclude recoverable errors",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: serverConfig5,
				Config:       PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			componentHealthConfig: &hcconfig.ComponentHealthConfig{
				IncludePermanent: true,
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
					},
				},
				{
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					queryParams:             "pipeline=traces",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					queryParams:             "pipeline=metrics",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:             "pipeline=traces",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewEvent(componentstatus.StatusOK),
						)
					},
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					queryParams:             "pipeline=metrics",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:             "pipeline=traces",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
					},
				},
				{
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
					},
				},
				{
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
					},
				},
			},
		},
		{
			name:         "include permanent and exclude recoverable errors - verbose",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: serverConfig6,
				Config:       PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			componentHealthConfig: &hcconfig.ComponentHealthConfig{
				IncludePermanent: true,
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusStarting,
								nestedStatus: componentStatusPipelineTracesStarting,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStarting,
								nestedStatus: componentStatusPipelineMetricsStarting,
							},
						},
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineMetricsOK,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineMetricsOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					eventually:         true,
					queryParams:        "verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy: true,
								status:  componentstatus.StatusRecoverableError,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:metrics/in": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"exporter:metrics/out": {
										healthy: true,
										status:  componentstatus.StatusRecoverableError,
										err:     assert.AnError,
									},
								},
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:metrics/in": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"exporter:metrics/out": {
								healthy: true,
								status:  componentstatus.StatusRecoverableError,
								err:     assert.AnError,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineTracesOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewEvent(componentstatus.StatusOK),
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineMetricsOK,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineMetricsOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy: false,
								status:  componentstatus.StatusPermanentError,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:metrics/in": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"exporter:metrics/out": {
										healthy: false,
										status:  componentstatus.StatusPermanentError,
										err:     assert.AnError,
									},
								},
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:metrics/in": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"exporter:metrics/out": {
								healthy: false,
								status:  componentstatus.StatusPermanentError,
							},
						},
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusStopping,
								nestedStatus: componentStatusPipelineTracesStopping,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStopping,
								nestedStatus: componentStatusPipelineMetricsStopping,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopping,
						nestedStatus: componentStatusPipelineTracesStopping,
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopping,
						nestedStatus: componentStatusPipelineMetricsStopping,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
						)
					},
					queryParams:        "",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusStopped,
								nestedStatus: componentStatusPipelineTracesStopped,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStopped,
								nestedStatus: componentStatusPipelineMetricsStopped,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopped,
						nestedStatus: componentStatusPipelineTracesStopped,
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopped,
						nestedStatus: componentStatusPipelineMetricsStopped,
					},
				},
			},
		},
		{
			name:         "include permanent and recoverable errors",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: serverConfig7,
				Config:       PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			componentHealthConfig: &hcconfig.ComponentHealthConfig{
				IncludePermanent:   true,
				IncludeRecoverable: true,
				RecoveryDuration:   2 * time.Millisecond,
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
					},
				},
				{
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					queryParams:             "pipeline=traces",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					queryParams:             "pipeline=metrics",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					eventually:         true,
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:             "pipeline=traces",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewEvent(componentstatus.StatusOK),
						)
					},
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					queryParams:             "pipeline=metrics",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
					},
				},
				{
					queryParams:             "pipeline=traces",
					expectedStatusCode:      http.StatusOK,
					expectedComponentStatus: componentStatusOK,
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
					},
				},
				{
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
					},
				},
				{
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
					},
				},
			},
		},
		{
			name:         "include permanent and recoverable errors - verbose",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: serverConfig8,
				Config:       PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			componentHealthConfig: &hcconfig.ComponentHealthConfig{
				IncludePermanent:   true,
				IncludeRecoverable: true,
				RecoveryDuration:   2 * time.Millisecond,
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStarting,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusStarting,
								nestedStatus: componentStatusPipelineTracesStarting,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStarting,
								nestedStatus: componentStatusPipelineMetricsStarting,
							},
						},
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineMetricsOK,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineMetricsOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					eventually:         true,
					queryParams:        "verbose",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy: false,
								status:  componentstatus.StatusRecoverableError,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:metrics/in": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"exporter:metrics/out": {
										healthy: false,
										status:  componentstatus.StatusRecoverableError,
										err:     assert.AnError,
									},
								},
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusRecoverableError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:metrics/in": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"exporter:metrics/out": {
								healthy: false,
								status:  componentstatus.StatusRecoverableError,
								err:     assert.AnError,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineTracesOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewEvent(componentstatus.StatusOK),
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineMetricsOK,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusOK,
						nestedStatus: componentStatusPipelineMetricsOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy: false,
								status:  componentstatus.StatusPermanentError,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:metrics/in": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  componentstatus.StatusOK,
									},
									"exporter:metrics/out": {
										healthy: false,
										status:  componentstatus.StatusPermanentError,
										err:     assert.AnError,
									},
								},
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  componentstatus.StatusPermanentError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:metrics/in": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  componentstatus.StatusOK,
							},
							"exporter:metrics/out": {
								healthy: false,
								status:  componentstatus.StatusPermanentError,
							},
						},
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
						)
					},
					queryParams:        "verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopping,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusStopping,
								nestedStatus: componentStatusPipelineTracesStopping,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStopping,
								nestedStatus: componentStatusPipelineMetricsStopping,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopping,
						nestedStatus: componentStatusPipelineTracesStopping,
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopping,
						nestedStatus: componentStatusPipelineMetricsStopping,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
						)
					},
					queryParams:        "",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusStopped,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusStopped,
								nestedStatus: componentStatusPipelineTracesStopped,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusStopped,
								nestedStatus: componentStatusPipelineMetricsStopped,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopped,
						nestedStatus: componentStatusPipelineTracesStopped,
					},
				},
				{
					queryParams:        "pipeline=metrics&verbose",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy:      true,
						status:       componentstatus.StatusStopped,
						nestedStatus: componentStatusPipelineMetricsStopped,
					},
				},
			},
		},
		{
			name:         "pipeline nonexistent",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: serverConfig9,
				Config:       PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					queryParams:        "pipeline=nonexistent",
					expectedStatusCode: http.StatusNotFound,
				},
			},
		},
		{
			name:         "verbose explicitly false",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: serverConfig10,
				Config:       PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
						testhelpers.SeedAggregator(server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					queryParams:        "verbose=false",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusOK,
					},
				},
			},
		},
		{
			name:         "verbose explicitly true",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: serverConfig11,
				Config:       PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
						testhelpers.SeedAggregator(server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					queryParams:        "verbose=true",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  componentstatus.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineTracesOK,
							},
							"pipeline:metrics": {
								healthy:      true,
								status:       componentstatus.StatusOK,
								nestedStatus: componentStatusPipelineMetricsOK,
							},
						},
					},
				},
			},
		},
		{
			name:         "status disabled",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: serverConfig12,
				Config:       PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: false,
				},
			},
			teststeps: []teststep{
				{
					expectedStatusCode: http.StatusNotFound,
				},
			},
		},
		{
			name: "legacy - default response",
			legacyConfig: LegacyConfig{
				ServerConfig: serverConfig13,
				Path:         "/status",
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       expectedBodyNotReady,
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       expectedBodyNotReady,
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedBody:       expectedBodyReady,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedBody:       expectedBodyReady,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewEvent(componentstatus.StatusOK),
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedBody:       expectedBodyReady,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedBody:       expectedBodyReady,
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewFatalErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       expectedBodyNotReady,
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       expectedBodyNotReady,
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       expectedBodyNotReady,
				},
			},
		},
		{
			name: "legacy - custom response",
			legacyConfig: LegacyConfig{
				ServerConfig: serverConfig14,
				Path:         "/status",
				ResponseBody: &ResponseBodyConfig{Healthy: "ALL OK", Unhealthy: "NOT OK"},
			},
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStarting,
						)
						testhelpers.SeedAggregator(server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStarting,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusOK,
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedBody:       "ALL OK",
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedBody:       "ALL OK",
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewEvent(componentstatus.StatusOK),
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedBody:       "ALL OK",
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewPermanentErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedBody:       "ALL OK",
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							metrics.ExporterID,
							componentstatus.NewFatalErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopping,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopping,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							traces.InstanceIDs(),
							componentstatus.StatusStopped,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							metrics.InstanceIDs(),
							componentstatus.StatusStopped,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedBody:       "NOT OK",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server = NewServer(
				tc.config,
				tc.legacyConfig,
				tc.componentHealthConfig,
				componenttest.NewNopTelemetrySettings(),
				status.NewAggregator(internalhelpers.ErrPriority(tc.componentHealthConfig)),
			)

			require.NoError(t, server.Start(t.Context(), componenttest.NewNopHost()))
			ts := httptest.NewServer(server.mux)

			// Ensure cleanup happens in the correct order
			defer func() {
				ts.Close()
				http.DefaultTransport.(*http.Transport).CloseIdleConnections()
				require.NoError(t, server.Shutdown(t.Context()))
			}()

			var path string
			if tc.legacyConfig.UseV2 {
				path = tc.config.Status.Path
			} else {
				path = tc.legacyConfig.Path
			}
			url := ts.URL + path

			// Create a custom client with aggressive timeouts
			transport := &http.Transport{DisableKeepAlives: true, MaxConnsPerHost: 1}
			defer transport.CloseIdleConnections()
			client := &http.Client{
				Timeout:   100 * time.Millisecond,
				Transport: transport,
			}

			// Wait for server to be ready before starting test steps
			require.Eventually(t, func() bool {
				resp, err := client.Get(ts.URL)
				if err != nil {
					return false
				}
				resp.Body.Close()
				return true
			}, time.Second, 10*time.Millisecond, "server failed to become ready")

			for _, ts := range tc.teststeps {
				if ts.step != nil {
					ts.step()
				}

				stepURL := url
				if ts.queryParams != "" {
					stepURL = fmt.Sprintf("%s?%s", stepURL, ts.queryParams)
				}

				var err error
				var resp *http.Response
				var body []byte

				if ts.eventually {
					assert.EventuallyWithT(t, func(tt *assert.CollectT) {
						localResp, localErr := client.Get(stepURL)
						require.NoError(tt, localErr)
						defer localResp.Body.Close()
						assert.Equal(tt, ts.expectedStatusCode, localResp.StatusCode)
					}, time.Second, 10*time.Millisecond)
					// Make a final request to get the body for assertions
					resp, err = client.Get(stepURL)
					require.NoError(t, err)
					body, err = io.ReadAll(resp.Body)
					require.NoError(t, err)
					require.NoError(t, resp.Body.Close())
				} else {
					resp, err = client.Get(stepURL)
					require.NoError(t, err)
					body, err = io.ReadAll(resp.Body)
					require.NoError(t, err)
					require.NoError(t, resp.Body.Close())
					assert.Equal(t, ts.expectedStatusCode, resp.StatusCode)
				}

				assert.Contains(t, string(body), ts.expectedBody)

				if ts.expectedComponentStatus != nil {
					st := &serializableStatus{}
					require.NoError(t, json.Unmarshal(body, st))
					if strings.Contains(ts.queryParams, "verbose") && !strings.Contains(ts.queryParams, "verbose=false") {
						assertStatusDetailed(t, ts.expectedComponentStatus, st)
						continue
					}
					assertStatusSimple(t, ts.expectedComponentStatus, st)
				}
			}
		})
	}
}

func assertStatusDetailed(
	t *testing.T,
	expected *componentStatusExpectation,
	actual *serializableStatus,
) {
	assert.Equal(t, expected.healthy, actual.Healthy)
	assert.Equal(t, expected.status, actual.Status(),
		"want: %s, got: %s", expected.status, actual.Status())
	if expected.err != nil {
		assert.Equal(t, expected.err.Error(), actual.Error)
	}
	assertNestedStatus(t, expected.nestedStatus, actual.ComponentStatuses)
}

func assertNestedStatus(
	t *testing.T,
	expected map[string]*componentStatusExpectation,
	actual map[string]*serializableStatus,
) {
	for k, expectation := range expected {
		st, ok := actual[k]
		require.True(t, ok, "status for key: %s not found", k)
		assert.Equal(t, expectation.healthy, st.Healthy)
		assert.Equal(t, expectation.status, st.Status(),
			"want: %s, got: %s", expectation.status, st.Status())
		if expectation.err != nil {
			assert.Equal(t, expectation.err.Error(), st.Error)
		}
		assertNestedStatus(t, expectation.nestedStatus, st.ComponentStatuses)
	}
}

func assertStatusSimple(
	t *testing.T,
	expected *componentStatusExpectation,
	actual *serializableStatus,
) {
	assert.Equal(t, expected.status, actual.Status())
	assert.Equal(t, expected.healthy, actual.Healthy)
	if expected.err != nil {
		assert.Equal(t, expected.err.Error(), actual.Error)
	}
	assert.Nil(t, actual.ComponentStatuses)
}

func TestConfig(t *testing.T) {
	// These goroutines are part of the http.Client's connection pool management.
	// They don't accept context.Context and are managed by the transport's lifecycle,
	// not our test lifecycle. They'll be cleaned up when the transport is garbage collected.
	opts := []goleak.Option{
		goleak.IgnoreCurrent(),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	}
	goleak.VerifyNone(t, opts...)

	var server *Server
	confMap, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	confJSON, err := os.ReadFile(filepath.Clean(filepath.Join("testdata", "config.json")))
	require.NoError(t, err)

	tcServerConfig1 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	tcServerConfig1.WriteTimeout = 0
	tcServerConfig1.ReadHeaderTimeout = 0
	tcServerConfig1.IdleTimeout = 0
	tcServerConfig1.KeepAlivesEnabled = false
	tcServerConfig1.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	tcServerConfig2 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	tcServerConfig2.WriteTimeout = 0
	tcServerConfig2.ReadHeaderTimeout = 0
	tcServerConfig2.IdleTimeout = 0
	tcServerConfig2.KeepAlivesEnabled = false
	tcServerConfig2.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}
	tcServerConfig3 := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	tcServerConfig3.WriteTimeout = 0
	tcServerConfig3.ReadHeaderTimeout = 0
	tcServerConfig3.IdleTimeout = 0
	tcServerConfig3.KeepAlivesEnabled = false
	tcServerConfig3.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  testutil.GetAvailableLocalAddress(t),
	}

	for _, tc := range []struct {
		name               string
		config             *Config
		setup              func()
		expectedStatusCode int
		expectedBody       []byte
	}{
		{
			name: "config not notified",
			config: &Config{
				ServerConfig: tcServerConfig1,
				Config: PathConfig{
					Enabled: true,
					Path:    "/config",
				},
				Status: PathConfig{
					Enabled: false,
				},
			},
			expectedStatusCode: http.StatusServiceUnavailable,
			expectedBody:       []byte{},
		},
		{
			name: "config notified",
			config: &Config{
				ServerConfig: tcServerConfig2,
				Config: PathConfig{
					Enabled: true,
					Path:    "/config",
				},
				Status: PathConfig{
					Enabled: false,
				},
			},
			setup: func() {
				require.NoError(t, server.NotifyConfig(t.Context(), confMap))
			},
			expectedStatusCode: http.StatusOK,
			expectedBody:       confJSON,
		},
		{
			name: "config disabled",
			config: &Config{
				ServerConfig: tcServerConfig3,
				Config: PathConfig{
					Enabled: false,
				},
				Status: PathConfig{
					Enabled: false,
				},
			},
			expectedStatusCode: http.StatusNotFound,
			expectedBody:       []byte("404 page not found\n"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server = NewServer(
				tc.config,
				LegacyConfig{UseV2: true},
				&hcconfig.ComponentHealthConfig{},
				componenttest.NewNopTelemetrySettings(),
				status.NewAggregator(status.PriorityPermanent),
			)

			require.NoError(t, server.Start(t.Context(), componenttest.NewNopHost()))
			ts := httptest.NewServer(server.mux)

			// Ensure cleanup happens in the correct order
			defer func() {
				ts.Close()
				require.NoError(t, server.Shutdown(t.Context()))
			}()

			// Use a single client for all requests in this test
			transport := &http.Transport{
				DisableKeepAlives:   true,
				MaxIdleConnsPerHost: -1,
				DisableCompression:  true,
				MaxConnsPerHost:     -1,
				IdleConnTimeout:     1 * time.Millisecond,
				TLSHandshakeTimeout: 1 * time.Millisecond,
			}
			client := &http.Client{Transport: transport}
			defer transport.CloseIdleConnections()

			url := ts.URL + tc.config.Config.Path

			if tc.setup != nil {
				tc.setup()
			}

			resp, err := client.Get(url)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedBody, body)
		})
	}
}

func TestStatusIncludesAttributesWhenEnabled(t *testing.T) {
	// These goroutines are part of the http.Client's connection pool management.
	// They don't accept context.Context and are managed by the transport's lifecycle,
	// not our test lifecycle. They'll be cleaned up when the transport is garbage collected.
	opts := []goleak.Option{
		goleak.IgnoreCurrent(),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	}
	goleak.VerifyNone(t, opts...)

	metrics := testhelpers.NewPipelineMetadata(pipeline.SignalMetrics)
	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)

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
		ServerConfig: serverConfig,
		Config:       PathConfig{Enabled: false},
		Status: PathConfig{
			Enabled:           true,
			Path:              "/status",
			IncludeAttributes: true,
		},
	}

	aggregator := status.NewAggregator(status.PriorityPermanent)
	server := NewServer(
		config,
		LegacyConfig{UseV2: true},
		nil,
		componenttest.NewNopTelemetrySettings(),
		aggregator,
	)

	require.NoError(t, server.Start(t.Context(), componenttest.NewNopHost()))
	ts := httptest.NewServer(server.mux)

	defer func() {
		ts.Close()
		require.NoError(t, server.Shutdown(t.Context()))
	}()

	testhelpers.SeedAggregator(aggregator, traces.InstanceIDs(), componentstatus.StatusOK)
	testhelpers.SeedAggregator(aggregator, metrics.InstanceIDs(), componentstatus.StatusOK)

	attrs := pcommon.NewMap()
	attrs.PutStr("error_msg", "not enough permissions to read cpu data")
	scrapers := attrs.PutEmptySlice("scrapers")
	for _, scraper := range []string{"cpu", "memory", "network"} {
		scrapers.AppendEmpty().SetStr(scraper)
	}

	aggregator.RecordStatus(
		metrics.ExporterID,
		componentstatus.NewEvent(
			componentstatus.StatusRecoverableError,
			componentstatus.WithError(assert.AnError),
			componentstatus.WithAttributes(attrs),
		),
	)

	transport := &http.Transport{DisableKeepAlives: true}
	client := &http.Client{Transport: transport}
	defer transport.CloseIdleConnections()

	resp, err := client.Get(ts.URL + config.Status.Path + "?verbose")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	st := &serializableStatus{}
	require.NoError(t, json.Unmarshal(body, st))

	expectedAttrs := map[string]any{
		"error_msg": "not enough permissions to read cpu data",
		"scrapers":  []any{"cpu", "memory", "network"},
	}

	assert.Equal(t, expectedAttrs, st.Attributes)

	metricsStatus, ok := st.ComponentStatuses["pipeline:"+metrics.PipelineID.String()]
	require.True(t, ok)
	assert.Equal(t, expectedAttrs, metricsStatus.Attributes)

	exporterKey := "exporter:" + metrics.ExporterID.ComponentID().String()
	exporterStatus, ok := metricsStatus.ComponentStatuses[exporterKey]
	require.True(t, ok)
	assert.Equal(t, expectedAttrs, exporterStatus.Attributes)
}

func TestStatusNonVerboseIncludesAttributes(t *testing.T) {
	// These goroutines are part of the http.Client's connection pool management.
	opts := []goleak.Option{
		goleak.IgnoreCurrent(),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	}
	goleak.VerifyNone(t, opts...)

	metrics := testhelpers.NewPipelineMetadata(pipeline.SignalMetrics)

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
		ServerConfig: serverConfig,
		Config:       PathConfig{Enabled: false},
		Status: PathConfig{
			Enabled:           true,
			Path:              "/status",
			IncludeAttributes: true,
		},
	}

	aggregator := status.NewAggregator(status.PriorityPermanent)
	server := NewServer(
		config,
		LegacyConfig{UseV2: true},
		nil,
		componenttest.NewNopTelemetrySettings(),
		aggregator,
	)

	require.NoError(t, server.Start(t.Context(), componenttest.NewNopHost()))
	ts := httptest.NewServer(server.mux)

	defer func() {
		ts.Close()
		require.NoError(t, server.Shutdown(t.Context()))
	}()

	testhelpers.SeedAggregator(aggregator, metrics.InstanceIDs(), componentstatus.StatusOK)

	attrs := pcommon.NewMap()
	attrs.PutStr("error_msg", "test error")
	aggregator.RecordStatus(
		metrics.ExporterID,
		componentstatus.NewEvent(
			componentstatus.StatusRecoverableError,
			componentstatus.WithError(assert.AnError),
			componentstatus.WithAttributes(attrs),
		),
	)

	transport := &http.Transport{DisableKeepAlives: true}
	client := &http.Client{Transport: transport}
	defer transport.CloseIdleConnections()

	// Request with include_attributes enabled - attributes should be included
	resp, err := client.Get(ts.URL + config.Status.Path)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	st := &serializableStatus{}
	require.NoError(t, json.Unmarshal(body, st))

	expectedAttrs := map[string]any{
		"error_msg": "test error",
	}
	assert.Equal(t, expectedAttrs, st.Attributes)
}

func TestStatusExcludesAttributesWhenConfigDisabled(t *testing.T) {
	// These goroutines are part of the http.Client's connection pool management.
	opts := []goleak.Option{
		goleak.IgnoreCurrent(),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
	}
	goleak.VerifyNone(t, opts...)

	metrics := testhelpers.NewPipelineMetadata(pipeline.SignalMetrics)

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
		ServerConfig: serverConfig,
		Config:       PathConfig{Enabled: false},
		Status: PathConfig{
			Enabled:           true,
			Path:              "/status",
			IncludeAttributes: false,
		},
	}

	aggregator := status.NewAggregator(status.PriorityPermanent)
	server := NewServer(
		config,
		LegacyConfig{UseV2: true},
		nil,
		componenttest.NewNopTelemetrySettings(),
		aggregator,
	)

	require.NoError(t, server.Start(t.Context(), componenttest.NewNopHost()))
	ts := httptest.NewServer(server.mux)

	defer func() {
		ts.Close()
		require.NoError(t, server.Shutdown(t.Context()))
	}()

	testhelpers.SeedAggregator(aggregator, metrics.InstanceIDs(), componentstatus.StatusOK)

	attrs := pcommon.NewMap()
	attrs.PutStr("error_msg", "test error")
	aggregator.RecordStatus(
		metrics.ExporterID,
		componentstatus.NewEvent(
			componentstatus.StatusRecoverableError,
			componentstatus.WithError(assert.AnError),
			componentstatus.WithAttributes(attrs),
		),
	)

	transport := &http.Transport{DisableKeepAlives: true}
	client := &http.Client{Transport: transport}
	defer transport.CloseIdleConnections()

	// Request with include_attributes=false - attributes should not be included
	resp, err := client.Get(ts.URL + config.Status.Path)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	st := &serializableStatus{}
	require.NoError(t, json.Unmarshal(body, st))

	// Attributes should be empty map when include_attributes is false
	assert.Empty(t, st.Attributes)
}
