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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/status"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextensionv2/internal/testhelpers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

type componentStatusExpectation struct {
	healthy      bool
	status       component.Status
	err          error
	nestedStatus map[string]*componentStatusExpectation
}

type teststep struct {
	step                    func()
	queryParams             string
	eventually              bool
	expectedStatusCode      int
	expectedComponentStatus *componentStatusExpectation
}

func TestStatus(t *testing.T) {
	// server and pipeline are reassigned before each test and are available for
	// use in the teststeps
	var server *Server
	var pipelines map[string]*testhelpers.PipelineMetadata

	tests := []struct {
		name      string
		settings  *Settings
		pipelines map[string]*testhelpers.PipelineMetadata
		teststeps []teststep
	}{
		{
			name: "Collector Status",
			settings: &Settings{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathSettings{Enabled: false},
				Status: StatusSettings{
					Detailed: false,
					PathSettings: PathSettings{
						Enabled: true,
						Path:    "/status",
					},
				},
			},
			pipelines: testhelpers.NewPipelines("traces"),
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(server.aggregator,
							pipelines["traces"].InstanceIDs(),
							component.StatusStarting,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusStarting,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							pipelines["traces"].InstanceIDs(),
							component.StatusOK,
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							pipelines["traces"].ExporterID,
							component.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					eventually:         true,
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  component.StatusRecoverableError,
						err:     assert.AnError,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							pipelines["traces"].ExporterID,
							component.NewStatusEvent(component.StatusOK),
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusOK,
					},
				},
			},
		},
		{
			name: "Collector Status - Detailed",
			settings: &Settings{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathSettings{Enabled: false},
				Status: StatusSettings{
					Detailed: true,
					PathSettings: PathSettings{
						Enabled: true,
						Path:    "/status",
					},
				},
			},
			pipelines: testhelpers.NewPipelines("traces"),
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							pipelines["traces"].InstanceIDs(),
							component.StatusStarting,
						)
					},
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusStarting,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy: true,
								status:  component.StatusStarting,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:traces/in": {
										healthy: true,
										status:  component.StatusStarting,
									},
									"processor:batch": {
										healthy: true,
										status:  component.StatusStarting,
									},
									"exporter:traces/out": {
										healthy: true,
										status:  component.StatusStarting,
									},
								},
							},
						},
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							pipelines["traces"].InstanceIDs(),
							component.StatusOK,
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy: true,
								status:  component.StatusOK,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:traces/in": {
										healthy: true,
										status:  component.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  component.StatusOK,
									},
									"exporter:traces/out": {
										healthy: true,
										status:  component.StatusOK,
									},
								},
							},
						},
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							pipelines["traces"].ExporterID,
							component.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					eventually:         true,
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  component.StatusRecoverableError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy: false,
								status:  component.StatusRecoverableError,
								err:     assert.AnError,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:traces/in": {
										healthy: true,
										status:  component.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  component.StatusOK,
									},
									"exporter:traces/out": {
										healthy: false,
										status:  component.StatusRecoverableError,
										err:     assert.AnError,
									},
								},
							},
						},
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							pipelines["traces"].ExporterID,
							component.NewStatusEvent(component.StatusOK),
						)
					},
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy: true,
								status:  component.StatusOK,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:traces/in": {
										healthy: true,
										status:  component.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  component.StatusOK,
									},
									"exporter:traces/out": {
										healthy: true,
										status:  component.StatusOK,
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Pipeline Status",
			settings: &Settings{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathSettings{Enabled: false},
				Status: StatusSettings{
					Detailed: false,
					PathSettings: PathSettings{
						Enabled: true,
						Path:    "/status",
					},
				},
			},
			pipelines: testhelpers.NewPipelines("traces"),
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							pipelines["traces"].InstanceIDs(),
							component.StatusStarting,
						)
					},
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusStarting,
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							pipelines["traces"].InstanceIDs(),
							component.StatusOK,
						)
					},
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusOK,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							pipelines["traces"].ExporterID,
							component.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					queryParams:        "pipeline=traces",
					eventually:         true,
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  component.StatusRecoverableError,
						err:     assert.AnError,
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							pipelines["traces"].ExporterID,
							component.NewStatusEvent(component.StatusOK),
						)
					},
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusOK,
					},
				},
			},
		},
		{
			name: "Pipeline Status - Detailed",
			settings: &Settings{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathSettings{Enabled: false},
				Status: StatusSettings{
					Detailed: true,
					PathSettings: PathSettings{
						Enabled: true,
						Path:    "/status",
					},
				},
			},
			pipelines: testhelpers.NewPipelines("traces"),
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(server.aggregator,
							pipelines["traces"].InstanceIDs(),
							component.StatusStarting,
						)
					},
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusServiceUnavailable,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusStarting,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:traces/in": {
								healthy: true,
								status:  component.StatusStarting,
							},
							"processor:batch": {
								healthy: true,
								status:  component.StatusStarting,
							},
							"exporter:traces/out": {
								healthy: true,
								status:  component.StatusStarting,
							},
						},
					},
				},
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							pipelines["traces"].InstanceIDs(),
							component.StatusOK,
						)
					},
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:traces/in": {
								healthy: true,
								status:  component.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  component.StatusOK,
							},
							"exporter:traces/out": {
								healthy: true,
								status:  component.StatusOK,
							},
						},
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							pipelines["traces"].ExporterID,
							component.NewRecoverableErrorEvent(assert.AnError),
						)
					},
					queryParams:        "pipeline=traces",
					eventually:         true,
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  component.StatusRecoverableError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:traces/in": {
								healthy: true,
								status:  component.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  component.StatusOK,
							},
							"exporter:traces/out": {
								healthy: false,
								status:  component.StatusRecoverableError,
								err:     assert.AnError,
							},
						},
					},
				},
				{
					step: func() {
						server.aggregator.RecordStatus(
							pipelines["traces"].ExporterID,
							component.NewStatusEvent(component.StatusOK),
						)
					},
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:traces/in": {
								healthy: true,
								status:  component.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  component.StatusOK,
							},
							"exporter:traces/out": {
								healthy: true,
								status:  component.StatusOK,
							},
						},
					},
				},
			},
		},
		{
			name: "Multiple Pipelines",
			settings: &Settings{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathSettings{Enabled: false},
				Status: StatusSettings{
					Detailed: true,
					PathSettings: PathSettings{
						Enabled: true,
						Path:    "/status",
					},
				},
			},
			pipelines: testhelpers.NewPipelines("traces", "metrics"),
			teststeps: []teststep{
				{
					step: func() {
						// traces will be StatusOK
						testhelpers.SeedAggregator(
							server.aggregator,
							pipelines["traces"].InstanceIDs(),
							component.StatusOK,
						)
						testhelpers.SeedAggregator(
							server.aggregator,
							pipelines["metrics"].InstanceIDs(),
							component.StatusOK,
						)
						// metrics and overall status will be PermanentError
						server.aggregator.RecordStatus(
							pipelines["metrics"].ExporterID,
							component.NewPermanentErrorEvent(assert.AnError),
						)
					},
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  component.StatusPermanentError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"pipeline:traces": {
								healthy: true,
								status:  component.StatusOK,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:traces/in": {
										healthy: true,
										status:  component.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  component.StatusOK,
									},
									"exporter:traces/out": {
										healthy: true,
										status:  component.StatusOK,
									},
								},
							},
							"pipeline:metrics": {
								healthy: false,
								status:  component.StatusPermanentError,
								err:     assert.AnError,
								nestedStatus: map[string]*componentStatusExpectation{
									"receiver:metrics/in": {
										healthy: true,
										status:  component.StatusOK,
									},
									"processor:batch": {
										healthy: true,
										status:  component.StatusOK,
									},
									"exporter:metrics/out": {
										healthy: false,
										status:  component.StatusPermanentError,
										err:     assert.AnError,
									},
								},
							},
						},
					},
				},
				{
					queryParams:        "pipeline=traces",
					expectedStatusCode: http.StatusOK,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: true,
						status:  component.StatusOK,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:traces/in": {
								healthy: true,
								status:  component.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  component.StatusOK,
							},
							"exporter:traces/out": {
								healthy: true,
								status:  component.StatusOK,
							},
						},
					},
				},
				{
					queryParams:        "pipeline=metrics",
					expectedStatusCode: http.StatusInternalServerError,
					expectedComponentStatus: &componentStatusExpectation{
						healthy: false,
						status:  component.StatusPermanentError,
						err:     assert.AnError,
						nestedStatus: map[string]*componentStatusExpectation{
							"receiver:metrics/in": {
								healthy: true,
								status:  component.StatusOK,
							},
							"processor:batch": {
								healthy: true,
								status:  component.StatusOK,
							},
							"exporter:metrics/out": {
								healthy: false,
								status:  component.StatusPermanentError,
								err:     assert.AnError,
							},
						},
					},
				},
			},
		},
		{
			name: "Pipeline Non-existent",
			settings: &Settings{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathSettings{Enabled: false},
				Status: StatusSettings{
					Detailed: false,
					PathSettings: PathSettings{
						Enabled: true,
						Path:    "/status",
					},
				},
			},
			pipelines: testhelpers.NewPipelines("traces"),
			teststeps: []teststep{
				{
					step: func() {
						testhelpers.SeedAggregator(
							server.aggregator,
							pipelines["traces"].InstanceIDs(),
							component.StatusOK,
						)
					},
					queryParams:        "pipeline=nonexistent",
					expectedStatusCode: http.StatusNotFound,
				},
			},
		},
		{
			name: "Status Disabled",
			settings: &Settings{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathSettings{Enabled: false},
				Status: StatusSettings{
					PathSettings: PathSettings{
						Enabled: false,
					},
				},
			},
			teststeps: []teststep{
				{
					expectedStatusCode: http.StatusNotFound,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pipelines = tc.pipelines
			server = NewServer(
				tc.settings,
				componenttest.NewNopTelemetrySettings(),
				20*time.Millisecond,
				status.NewAggregator(),
			)

			require.NoError(t, server.Start(context.Background(), componenttest.NewNopHost()))
			defer func() { require.NoError(t, server.Shutdown(context.Background())) }()
			require.NoError(t, server.Ready())

			client := &http.Client{}
			url := fmt.Sprintf("http://%s%s", tc.settings.Endpoint, tc.settings.Status.Path)

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

				if ts.expectedComponentStatus != nil {
					body, err := io.ReadAll(resp.Body)
					require.NoError(t, err)

					st := &serializableStatus{}
					require.NoError(t, json.Unmarshal(body, st))

					if tc.settings.Status.Detailed {
						assertStatusDetailed(t, ts.expectedComponentStatus, st)
						continue
					}

					assertStatusSimple(t, ts.expectedComponentStatus, st)
				}
			}
		})
	}
}

func TestPipelineReady(t *testing.T) {
	settings := &Settings{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
		Config: PathSettings{Enabled: false},
		Status: StatusSettings{
			Detailed: false,
			PathSettings: PathSettings{
				Enabled: true,
				Path:    "/status",
			},
		},
	}
	server := NewServer(
		settings,
		componenttest.NewNopTelemetrySettings(),
		20*time.Millisecond,
		status.NewAggregator(),
	)

	require.NoError(t, server.Start(context.Background(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, server.Shutdown(context.Background())) }()

	client := &http.Client{}
	url := fmt.Sprintf("http://%s%s", settings.Endpoint, settings.Status.Path)
	traces := testhelpers.NewPipelineMetadata("traces")

	for _, ts := range []struct {
		step               func()
		expectedStatusCode int
		ready              bool
		notReady           bool
	}{
		{
			step: func() {
				testhelpers.SeedAggregator(
					server.aggregator,
					traces.InstanceIDs(),
					component.StatusOK,
				)
			},
			expectedStatusCode: http.StatusServiceUnavailable,
		},
		{
			expectedStatusCode: http.StatusOK,
			ready:              true,
		},
		{
			expectedStatusCode: http.StatusServiceUnavailable,
			notReady:           true,
		},
	} {
		if ts.step != nil {
			ts.step()
		}

		if ts.ready {
			require.NoError(t, server.Ready())
		}

		if ts.notReady {
			require.NoError(t, server.NotReady())
		}

		resp, err := client.Get(url)
		require.NoError(t, err)
		assert.Equal(t, ts.expectedStatusCode, resp.StatusCode)
	}
}

func assertStatusDetailed(
	t *testing.T,
	expected *componentStatusExpectation,
	actual *serializableStatus,
) {
	assert.Equal(t, expected.healthy, actual.Healthy)
	assert.Equal(t, expected.status, actual.Status())
	assert.Equal(t, !component.StatusIsError(expected.status), actual.Healthy)
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
		assert.Equal(t, expectation.status, st.Status())
		assert.Equal(t, !component.StatusIsError(expectation.status), st.Healthy)
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
	assert.Equal(t, !component.StatusIsError(expected.status), actual.Healthy)
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
		settings           *Settings
		setup              func()
		expectedStatusCode int
		expectedBody       []byte
	}{
		{
			name: "config not notified",
			settings: &Settings{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathSettings{
					Enabled: true,
					Path:    "/config",
				},
				Status: StatusSettings{
					PathSettings: PathSettings{
						Enabled: false,
					},
				},
			},
			expectedStatusCode: http.StatusServiceUnavailable,
			expectedBody:       []byte{},
		},
		{
			name: "config notified",
			settings: &Settings{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathSettings{
					Enabled: true,
					Path:    "/config",
				},
				Status: StatusSettings{
					PathSettings: PathSettings{
						Enabled: false,
					},
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
			settings: &Settings{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: testutil.GetAvailableLocalAddress(t),
				},
				Config: PathSettings{
					Enabled: false,
				},
				Status: StatusSettings{
					PathSettings: PathSettings{
						Enabled: false,
					},
				},
			},
			expectedStatusCode: http.StatusNotFound,
			expectedBody:       []byte("404 page not found\n"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server = NewServer(
				tc.settings,
				componenttest.NewNopTelemetrySettings(),
				20*time.Millisecond,
				status.NewAggregator(),
			)

			require.NoError(t, server.Start(context.Background(), componenttest.NewNopHost()))
			defer func() { require.NoError(t, server.Shutdown(context.Background())) }()

			client := &http.Client{}
			url := fmt.Sprintf("http://%s%s", tc.settings.Endpoint, tc.settings.Config.Path)

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
