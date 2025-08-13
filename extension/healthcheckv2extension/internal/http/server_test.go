// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/common"
	internalhelpers "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension/internal/testhelpers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
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
	var server *Server
	traces := testhelpers.NewPipelineMetadata(pipeline.SignalTraces)
	metrics := testhelpers.NewPipelineMetadata(pipeline.SignalMetrics)

	tests := []struct {
		name                  string
		config                *Config
		legacyConfig          LegacyConfig
		componentHealthConfig *common.ComponentHealthConfig
		teststeps             []teststep
	}{
		{
			name:         "exclude recoverable and permanent errors",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{Enabled: false},
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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{Enabled: false},
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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			componentHealthConfig: &common.ComponentHealthConfig{
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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			componentHealthConfig: &common.ComponentHealthConfig{
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
			name:         "include permanent and exclude recoverable errors",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			componentHealthConfig: &common.ComponentHealthConfig{
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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			componentHealthConfig: &common.ComponentHealthConfig{
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
			name:         "include permanent and recoverable errors",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			componentHealthConfig: &common.ComponentHealthConfig{
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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{Enabled: false},
				Status: PathConfig{
					Enabled: true,
					Path:    "/status",
				},
			},
			componentHealthConfig: &common.ComponentHealthConfig{
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
			name:         "pipeline nonexistent",
			legacyConfig: LegacyConfig{UseV2: true},
			config: &Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{Enabled: false},
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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{Enabled: false},
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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{Enabled: false},
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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{Enabled: false},
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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Path: "/status",
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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
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

			require.NoError(t, server.Start(context.Background(), componenttest.NewNopHost()))
			defer func() { require.NoError(t, server.Shutdown(context.Background())) }()

			var url string
			if tc.legacyConfig.UseV2 {
				url = fmt.Sprintf("http://%s%s", tc.config.Endpoint, tc.config.Status.Path)
			} else {
				url = fmt.Sprintf("http://%s%s", tc.legacyConfig.Endpoint, tc.legacyConfig.Path)
			}

			client := &http.Client{}

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

				if ts.eventually {
					assert.Eventually(t, func() bool {
						resp, err = client.Get(stepURL)
						require.NoError(t, err)
						return ts.expectedStatusCode == resp.StatusCode
					}, time.Second, 10*time.Millisecond)
				} else {
					resp, err = client.Get(stepURL)
					require.NoError(t, err)
					assert.Equal(t, ts.expectedStatusCode, resp.StatusCode)
				}

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)

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
	var server *Server
	confMap, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	confJSON, err := os.ReadFile(filepath.Clean(filepath.Join("testdata", "config.json")))
	require.NoError(t, err)

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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
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
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathConfig{
					Enabled: true,
					Path:    "/config",
				},
				Status: PathConfig{
					Enabled: false,
				},
			},
			setup: func() {
				require.NoError(t, server.NotifyConfig(context.Background(), confMap))
			},
			expectedStatusCode: http.StatusOK,
			expectedBody:       confJSON,
		},
		{
			name: "config disabled",
			config: &Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
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
				&common.ComponentHealthConfig{},
				componenttest.NewNopTelemetrySettings(),
				status.NewAggregator(status.PriorityPermanent),
			)

			require.NoError(t, server.Start(context.Background(), componenttest.NewNopHost()))
			defer func() { require.NoError(t, server.Shutdown(context.Background())) }()

			client := &http.Client{}
			url := fmt.Sprintf("http://%s%s", tc.config.Endpoint, tc.config.Config.Path)

			if tc.setup != nil {
				tc.setup()
			}

			resp, err := client.Get(url)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedBody, body)
		})
	}
}
