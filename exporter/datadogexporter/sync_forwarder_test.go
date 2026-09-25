// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix && !solaris

package datadogexporter

import (
	"bytes"
	"compress/zlib"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DataDog/agent-payload/v5/gogen"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/serializerexporter"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

// syncForwarderIntake is an httptest.Server that counts requests and returns a
// fixed HTTP status code, letting tests simulate Datadog intake responses.
type syncForwarderIntake struct {
	*httptest.Server
	requests atomic.Int64
	status   int
}

func newSyncForwarderIntake(status int) *syncForwarderIntake {
	si := &syncForwarderIntake{status: status}
	si.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si.requests.Add(1)
		_, _ = io.ReadAll(r.Body)
		_ = r.Body.Close()
		w.WriteHeader(si.status)
	}))
	return si
}

// singleGaugeMetrics builds a minimal one-point pmetric.Metrics payload.
func singleGaugeMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("test.sync_forwarder.gauge")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(1.0)
	return md
}

func azureAppServiceMetrics(instanceIDs ...string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for _, instanceID := range instanceIDs {
		rm := md.ResourceMetrics().AppendEmpty()
		attrs := rm.Resource().Attributes()
		attrs.PutStr("cloud.platform", "azure.app_service")
		attrs.PutStr("service.name", "my-app")
		attrs.PutStr("cloud.account.id", "sub-123")
		attrs.PutStr("azure.resource_group.name", "my-rg")
		attrs.PutStr("azure.app_service.instance.id", instanceID)

		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetName("test.azure_app_service.gauge")
		dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetDoubleValue(1)
	}
	return md
}

func runningMetricTags(t *testing.T, body []byte, metricName string) [][]string {
	t.Helper()

	zr, err := zlib.NewReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer zr.Close()
	data, err := io.ReadAll(zr)
	require.NoError(t, err)

	payload := new(gogen.MetricPayload)
	require.NoError(t, payload.Unmarshal(data))

	var tagSets [][]string
	for _, series := range payload.GetSeries() {
		if series.GetMetric() != metricName {
			continue
		}
		assert.Equal(t, gogen.MetricPayload_GAUGE, series.GetType())
		require.Len(t, series.GetResources(), 1)
		assert.Equal(t, "host", series.GetResources()[0].GetType())
		assert.Empty(t, series.GetResources()[0].GetName())
		require.Len(t, series.GetPoints(), 1)
		assert.Equal(t, 1.0, series.GetPoints()[0].GetValue())
		tagSets = append(tagSets, series.GetTags())
	}
	return tagSets
}

type azureFunctionsResource struct {
	appName       string
	instanceID    string
	functionName  string
	resourceGroup string
}

func azureFunctionsMetrics(resources ...azureFunctionsResource) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for _, resource := range resources {
		rm := md.ResourceMetrics().AppendEmpty()
		attrs := rm.Resource().Attributes()
		attrs.PutStr("cloud.platform", "azure.functions")
		attrs.PutStr("service.name", resource.appName)
		attrs.PutStr("cloud.account.id", "sub-123")
		if resource.resourceGroup != "" {
			attrs.PutStr("azure.resource_group.name", resource.resourceGroup)
		}
		attrs.PutStr("faas.instance", resource.instanceID)
		attrs.PutStr("faas.name", resource.functionName)

		metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetName("azure.functions.requests")
		dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetDoubleValue(1)
	}
	return md
}

func azureFunctionsIdentityTags(tagSets [][]string) [][]string {
	identityTagSets := make([][]string, 0, len(tagSets))
	for _, tagSet := range tagSets {
		identityTags := make([]string, 0, 4)
		for _, tag := range tagSet {
			if strings.HasPrefix(tag, "instance:") ||
				strings.HasPrefix(tag, "name:") ||
				strings.HasPrefix(tag, "resource_group:") ||
				strings.HasPrefix(tag, "subscription_id:") {
				identityTags = append(identityTags, tag)
			}
		}
		identityTagSets = append(identityTagSets, identityTags)
	}
	return identityTagSets
}

func completeGCPServerlessAttributes(platform string) map[string]any {
	return map[string]any{
		"cloud.provider":   "gcp",
		"cloud.platform":   platform,
		"cloud.account.id": "project-1",
		"cloud.region":     "us-central1",
		"faas.name":        "my-service",
		"faas.instance":    "instance-1",
		"faas.version":     "revision-1",
		"host.name":        "resource-host",
	}
}

func gcpServerlessMetrics(t *testing.T, resourceAttrs map[string]any) pmetric.Metrics {
	t.Helper()
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	require.NoError(t, rm.Resource().Attributes().FromRaw(resourceAttrs))
	metric := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("test.gcp_serverless.gauge")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(1)
	return md
}

func gcpServerlessIdentityTags(tagSets [][]string) [][]string {
	identityTagSets := make([][]string, 0, len(tagSets))
	for _, tagSet := range tagSets {
		identityTags := make([]string, 0, 4)
		for _, tag := range tagSet {
			if strings.HasPrefix(tag, "instance:") ||
				strings.HasPrefix(tag, "service_name:") ||
				strings.HasPrefix(tag, "project_id:") ||
				strings.HasPrefix(tag, "location:") {
				identityTags = append(identityTags, tag)
			}
		}
		slices.Sort(identityTags)
		identityTagSets = append(identityTagSets, identityTags)
	}
	return identityTagSets
}

// setSyncForwarderGate enables or disables the UseSyncForwarder gate and
// registers a t.Cleanup that restores the previous state.
func setSyncForwarderGate(t *testing.T, enabled bool) {
	t.Helper()
	prev := serializerexporter.IsSyncForwarderEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set("datadog.serializerexporter.UseSyncForwarder", enabled))
	t.Cleanup(func() {
		_ = featuregate.GlobalRegistry().Set("datadog.serializerexporter.UseSyncForwarder", prev)
	})
}

// buildSyncForwarderExporter creates a datadogexporter via the serializer path
// pointed at intakeURL. The sending_queue is disabled so ConsumeMetrics is
// synchronous and send errors surface immediately to the caller.
func buildSyncForwarderExporter(t *testing.T, intakeURL string) exporter.Metrics {
	t.Helper()

	// Ensure the serializer exporter path is active for this test. It is Beta
	// (enabled by default) but we pin it explicitly to guard against future
	// default changes.
	prevSer := metadata.ExporterDatadogexporterMetricexportserializerclientFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(
		metadata.ExporterDatadogexporterMetricexportserializerclientFeatureGate.ID(), true,
	))
	t.Cleanup(func() {
		_ = featuregate.GlobalRegistry().Set(
			metadata.ExporterDatadogexporterMetricexportserializerclientFeatureGate.ID(), prevSer,
		)
	})

	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			FailOnInvalidKey: false,
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: intakeURL,
			},
			DeltaTTL: 3600,
			HistConfig: datadogconfig.HistogramConfig{
				Mode:             datadogconfig.HistogramModeDistributions,
				SendAggregations: false,
			},
			SumConfig: datadogconfig.SumConfig{
				CumulativeMonotonicMode: datadogconfig.CumulativeMonotonicSumModeToDelta,
			},
		},
		HostMetadata: datadogconfig.HostMetadataConfig{
			Enabled: false,
		},
		HostnameDetectionTimeout: 50 * time.Millisecond,
		// Disable the OTel sending_queue so ConsumeMetrics blocks until the
		// send completes and any error is returned inline to the caller.
		QueueSettings: configoptional.None[exporterhelper.QueueBatchConfig](),
	}

	params := exportertest.NewNopSettings(metadata.Type)
	f := NewFactory()

	exp, err := f.CreateMetrics(t.Context(), params, cfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { _ = exp.Shutdown(t.Context()) })
	return exp
}

func TestAzureAppServiceRunningMetrics(t *testing.T) {
	tests := []struct {
		name        string
		instanceIDs []string
	}{
		{name: "complete identity", instanceIDs: []string{"instance-1"}},
		{name: "distinct resources", instanceIDs: []string{"instance-1", "instance-2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setSyncForwarderGate(t, true)

			seriesRecorder := &testutil.HTTPRequestRecorderWithChan{
				Pattern: testutil.MetricV2Endpoint,
				ReqChan: make(chan []byte, 1),
			}
			server := testutil.DatadogServerMock(seriesRecorder.HandlerFunc)
			defer server.Close()

			exp := buildSyncForwarderExporter(t, server.URL)
			require.NoError(t, exp.ConsumeMetrics(t.Context(), azureAppServiceMetrics(tt.instanceIDs...)))

			var body []byte
			select {
			case body = <-seriesRecorder.ReqChan:
			case <-time.After(10 * time.Second):
				t.Fatal("timed out waiting for Datadog series payload")
			}

			got := runningMetricTags(t, body, "otel.datadog_exporter.metrics.running.azureappservices")
			require.Len(t, got, len(tt.instanceIDs))

			for _, instanceID := range tt.instanceIDs {
				var matchingTags []string
				for _, tags := range got {
					if slices.Contains(tags, "instance:"+instanceID) {
						matchingTags = tags
						break
					}
				}
				require.NotNil(t, matchingTags, "missing running metric for %s", instanceID)
				assert.Subset(t, matchingTags, []string{
					"instance:" + instanceID,
					"name:my-app",
					"resource_group:my-rg",
					"subscription_id:sub-123",
				})
			}
		})
	}
}

func TestAzureFunctionsRunningMetrics(t *testing.T) {
	tests := []struct {
		name      string
		resources []azureFunctionsResource
		wantTags  [][]string
	}{
		{
			name: "complete canonical identity",
			resources: []azureFunctionsResource{
				{appName: "my-app", instanceID: "instance-1", functionName: "function-a", resourceGroup: "my-rg"},
			},
			wantTags: [][]string{{
				"instance:instance-1",
				"name:my-app",
				"resource_group:my-rg",
				"subscription_id:sub-123",
			}},
		},
		{
			name: "two functions in one app instance",
			resources: []azureFunctionsResource{
				{appName: "my-app", instanceID: "instance-1", functionName: "function-a", resourceGroup: "my-rg"},
				{appName: "my-app", instanceID: "instance-1", functionName: "function-b", resourceGroup: "my-rg"},
			},
			wantTags: [][]string{{
				"instance:instance-1",
				"name:my-app",
				"resource_group:my-rg",
				"subscription_id:sub-123",
			}},
		},
		{
			name: "two app instances",
			resources: []azureFunctionsResource{
				{appName: "my-app", instanceID: "instance-1", functionName: "function-a", resourceGroup: "my-rg"},
				{appName: "my-app", instanceID: "instance-2", functionName: "function-a", resourceGroup: "my-rg"},
			},
			wantTags: [][]string{
				{"instance:instance-1", "name:my-app", "resource_group:my-rg", "subscription_id:sub-123"},
				{"instance:instance-2", "name:my-app", "resource_group:my-rg", "subscription_id:sub-123"},
			},
		},
		{
			name: "different apps with shared instance string",
			resources: []azureFunctionsResource{
				{appName: "first-app", instanceID: "shared-instance", functionName: "function-a", resourceGroup: "my-rg"},
				{appName: "second-app", instanceID: "shared-instance", functionName: "function-a", resourceGroup: "my-rg"},
			},
			wantTags: [][]string{
				{"instance:shared-instance", "name:first-app", "resource_group:my-rg", "subscription_id:sub-123"},
				{"instance:shared-instance", "name:second-app", "resource_group:my-rg", "subscription_id:sub-123"},
			},
		},
		{
			name: "incomplete identity",
			resources: []azureFunctionsResource{
				{appName: "my-app", instanceID: "instance-1", functionName: "function-a"},
			},
			wantTags: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setSyncForwarderGate(t, true)

			seriesRecorder := &testutil.HTTPRequestRecorderWithChan{
				Pattern: testutil.MetricV2Endpoint,
				ReqChan: make(chan []byte, 1),
			}
			server := testutil.DatadogServerMock(seriesRecorder.HandlerFunc)
			defer server.Close()

			exp := buildSyncForwarderExporter(t, server.URL)
			require.NoError(t, exp.ConsumeMetrics(t.Context(), azureFunctionsMetrics(tt.resources...)))

			var body []byte
			select {
			case body = <-seriesRecorder.ReqChan:
			case <-time.After(10 * time.Second):
				t.Fatal("timed out waiting for Datadog series payload")
			}

			got := runningMetricTags(t, body, "otel.datadog_exporter.metrics.running.azurefunctions")
			assert.ElementsMatch(t, tt.wantTags, azureFunctionsIdentityTags(got))
		})
	}
}

func TestGCPServerlessRunningMetrics(t *testing.T) {
	tests := []struct {
		name       string
		attributes map[string]any
		metricName string
		wantTags   [][]string
	}{
		{
			name:       "Cloud Run service",
			attributes: completeGCPServerlessAttributes("gcp_cloud_run"),
			metricName: "otel.datadog_exporter.metrics.running.cloudrun",
			wantTags: [][]string{{
				"instance:instance-1",
				"location:us-central1",
				"project_id:project-1",
				"service_name:my-service",
			}},
		},
		{
			name:       "Cloud Functions v2",
			attributes: completeGCPServerlessAttributes("gcp_cloud_functions"),
			metricName: "otel.datadog_exporter.metrics.running.cloudrunfunctions",
			wantTags: [][]string{{
				"instance:instance-1",
				"location:us-central1",
				"project_id:project-1",
				"service_name:my-service",
			}},
		},
		{
			name: "incomplete identity",
			attributes: func() map[string]any {
				attrs := completeGCPServerlessAttributes("gcp_cloud_run")
				delete(attrs, "faas.instance")
				return attrs
			}(),
		},
		{
			name: "Cloud Run job execution",
			attributes: func() map[string]any {
				attrs := completeGCPServerlessAttributes("gcp_cloud_run")
				attrs["gcp.cloud_run.job.execution"] = ""
				return attrs
			}(),
		},
		{
			name: "Cloud Run job task",
			attributes: func() map[string]any {
				attrs := completeGCPServerlessAttributes("gcp_cloud_run")
				attrs["gcp.cloud_run.job.task_index"] = int64(0)
				return attrs
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setSyncForwarderGate(t, true)

			seriesRecorder := &testutil.HTTPRequestRecorderWithChan{
				Pattern: testutil.MetricV2Endpoint,
				ReqChan: make(chan []byte, 1),
			}
			server := testutil.DatadogServerMock(seriesRecorder.HandlerFunc)
			defer server.Close()

			exp := buildSyncForwarderExporter(t, server.URL)
			require.NoError(t, exp.ConsumeMetrics(t.Context(), gcpServerlessMetrics(t, tt.attributes)))

			var body []byte
			select {
			case body = <-seriesRecorder.ReqChan:
			case <-time.After(10 * time.Second):
				t.Fatal("timed out waiting for Datadog series payload")
			}

			if tt.metricName == "" {
				assert.Empty(t, runningMetricTags(t, body, "otel.datadog_exporter.metrics.running.cloudrun"))
				assert.Empty(t, runningMetricTags(t, body, "otel.datadog_exporter.metrics.running.cloudrunfunctions"))
				return
			}
			got := runningMetricTags(t, body, tt.metricName)
			assert.ElementsMatch(t, tt.wantTags, gcpServerlessIdentityTags(got))
		})
	}
}

// TestSyncForwarder_PropagatesErrors verifies that when
// datadog.serializerexporter.UseSyncForwarder is enabled, a 5xx response from
// the Datadog intake is surfaced as an error from ConsumeMetrics, allowing
// OTel's exporterhelper retry/queue layer to observe and react to failures.
//
// Before this feature gate existed, the async DefaultForwarder enqueued
// requests internally and returned nil before the HTTP response was received,
// making failure invisible to exporterhelper (OTAGENT-1024).
func TestSyncForwarder_PropagatesErrors(t *testing.T) {
	setSyncForwarderGate(t, true)

	intake := newSyncForwarderIntake(http.StatusInternalServerError)
	defer intake.Close()

	exp := buildSyncForwarderExporter(t, intake.URL)

	err := exp.ConsumeMetrics(t.Context(), singleGaugeMetrics())
	assert.Error(t, err, "ConsumeMetrics should propagate send errors when the sync forwarder is enabled")
	assert.Positive(t, intake.requests.Load(), "the intake should have received at least one request")
}

// TestSyncForwarder_PermanentError verifies that a 4xx response from the
// Datadog intake is classified as a permanent (non-retryable) error when the
// sync forwarder is active, so OTel exporterhelper drops the batch rather than
// retrying indefinitely.
func TestSyncForwarder_PermanentError(t *testing.T) {
	setSyncForwarderGate(t, true)

	intake := newSyncForwarderIntake(http.StatusBadRequest)
	defer intake.Close()

	exp := buildSyncForwarderExporter(t, intake.URL)

	err := exp.ConsumeMetrics(t.Context(), singleGaugeMetrics())
	require.Error(t, err, "ConsumeMetrics should return an error for 4xx responses")
	assert.True(t, consumererror.IsPermanent(err),
		"a 400 Bad Request should be classified as permanent so exporterhelper drops rather than retries")
}

// TestDefaultForwarder_SwallowsErrors documents the legacy async forwarder
// behavior: ConsumeMetrics returns nil even when the intake returns 5xx.
// The async DefaultForwarder enqueues the request internally and returns
// before the HTTP round-trip completes, making the failure invisible to
// OTel exporterhelper. Enabling UseSyncForwarder fixes this (see above tests).
func TestDefaultForwarder_SwallowsErrors(t *testing.T) {
	setSyncForwarderGate(t, false) // explicitly use legacy async forwarder

	intake := newSyncForwarderIntake(http.StatusInternalServerError)
	defer intake.Close()

	exp := buildSyncForwarderExporter(t, intake.URL)

	err := exp.ConsumeMetrics(t.Context(), singleGaugeMetrics())
	assert.NoError(t, err,
		"legacy async forwarder silently swallows 5xx errors — ConsumeMetrics should return nil")
}

// buildExporterWithOpts is a generalized version of buildSyncForwarderExporter
// that accepts a custom queue config, retry config, and optional telemetry
// (pass nil to use nop telemetry).
func buildExporterWithOpts(
	t *testing.T,
	intakeURL string,
	tt *componenttest.Telemetry,
	queue configoptional.Optional[exporterhelper.QueueBatchConfig],
	retry configretry.BackOffConfig,
) exporter.Metrics {
	t.Helper()

	prevSer := metadata.ExporterDatadogexporterMetricexportserializerclientFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(
		metadata.ExporterDatadogexporterMetricexportserializerclientFeatureGate.ID(), true,
	))
	t.Cleanup(func() {
		_ = featuregate.GlobalRegistry().Set(
			metadata.ExporterDatadogexporterMetricexportserializerclientFeatureGate.ID(), prevSer,
		)
	})

	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key:              "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			FailOnInvalidKey: false,
		},
		Metrics: datadogconfig.MetricsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: intakeURL,
			},
			DeltaTTL: 3600,
			HistConfig: datadogconfig.HistogramConfig{
				Mode:             datadogconfig.HistogramModeDistributions,
				SendAggregations: false,
			},
			SumConfig: datadogconfig.SumConfig{
				CumulativeMonotonicMode: datadogconfig.CumulativeMonotonicSumModeToDelta,
			},
		},
		HostMetadata: datadogconfig.HostMetadataConfig{
			Enabled: false,
		},
		HostnameDetectionTimeout: 50 * time.Millisecond,
		QueueSettings:            queue,
		BackOffConfig:            retry,
	}

	params := exportertest.NewNopSettings(metadata.Type)
	if tt != nil {
		params.TelemetrySettings = tt.NewTelemetrySettings()
	}

	f := NewFactory()
	exp, err := f.CreateMetrics(t.Context(), params, cfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { _ = exp.Shutdown(t.Context()) })
	return exp
}

// otelCounterSum returns the cumulative sum of a named OTel internal counter
// recorded in tt. Returns 0 if the metric has not been recorded yet.
func otelCounterSum(t *testing.T, tt *componenttest.Telemetry, name string) int64 {
	t.Helper()
	m, err := tt.GetMetric(name)
	if err != nil {
		return 0 // metric not yet created = zero data points
	}
	sum, ok := m.Data.(metricdata.Sum[int64])
	require.True(t, ok, "metric %q should be a Sum[int64]", name)
	var total int64
	for _, dp := range sum.DataPoints {
		total += dp.Value
	}
	return total
}

// shortRetry returns a BackOffConfig that retries quickly, for tests that must
// exhaust the retry budget without waiting multiple minutes.
func shortRetry() configretry.BackOffConfig {
	return configretry.BackOffConfig{
		Enabled:         true,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     20 * time.Millisecond,
		MaxElapsedTime:  200 * time.Millisecond,
	}
}

// TestSyncForwarder_OTelRetry verifies that when UseSyncForwarder is enabled,
// a 5xx response causes ConsumeMetrics to surface a retryable error to OTel
// exporterhelper, which then fires the configured retry_on_failure policy.
// Multiple requests to the intake confirm that the OTel retry mechanism (not
// the async DefaultForwarder's internal retry) drove the re-sends.
func TestSyncForwarder_OTelRetry(t *testing.T) {
	setSyncForwarderGate(t, true)

	intake := newSyncForwarderIntake(http.StatusInternalServerError)
	defer intake.Close()

	exp := buildExporterWithOpts(t, intake.URL, nil,
		configoptional.None[exporterhelper.QueueBatchConfig](),
		shortRetry())

	err := exp.ConsumeMetrics(t.Context(), singleGaugeMetrics())
	assert.Error(t, err, "ConsumeMetrics should propagate error after retry budget is exhausted")
	assert.Greater(t, intake.requests.Load(), int64(1),
		"OTel retry should have produced more than one request to the intake")
}

// TestDefaultForwarder_NoOTelRetry documents that the legacy async forwarder
// does not trigger OTel-level retries: ConsumeMetrics returns nil immediately
// regardless of the retry_on_failure config, because the error is swallowed
// inside the DefaultForwarder before it can reach exporterhelper.
func TestDefaultForwarder_NoOTelRetry(t *testing.T) {
	setSyncForwarderGate(t, false)

	intake := newSyncForwarderIntake(http.StatusInternalServerError)
	defer intake.Close()

	exp := buildExporterWithOpts(t, intake.URL, nil,
		configoptional.None[exporterhelper.QueueBatchConfig](),
		shortRetry())

	err := exp.ConsumeMetrics(t.Context(), singleGaugeMetrics())
	assert.NoError(t, err,
		"async forwarder swallows errors so ConsumeMetrics returns nil; no OTel retry fires")
}

// TestSyncForwarder_QueueFull_EnqueueFailedCounter verifies that when
// UseSyncForwarder is enabled and the OTel sending queue fills up (because
// the sync forwarder's HTTP call blocks each worker), the
// otelcol_exporter_enqueue_failed_metric_points counter increments.
// This lets operators observe dropped metrics that would otherwise be silently
// lost inside the DefaultForwarder's internal queue.
func TestSyncForwarder_QueueFull_EnqueueFailedCounter(t *testing.T) {
	setSyncForwarderGate(t, true)

	// Server blocks all connections so queue workers stay busy.
	serverUnblock := make(chan struct{})
	intake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		_ = r.Body.Close()
		select {
		case <-serverUnblock:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer intake.Close()
	defer close(serverUnblock) // unblock on cleanup so Shutdown does not hang

	tt := componenttest.NewTelemetry()
	t.Cleanup(func() { _ = tt.Shutdown(t.Context()) })

	// Tiny queue: 2-item buffer, 1 consumer, non-blocking on overflow so
	// ConsumeMetrics returns quickly when the queue is full.
	queueCfg := exporterhelper.NewDefaultQueueConfig()
	queueCfg.QueueSize = 2
	queueCfg.NumConsumers = 1
	queueCfg.BlockOnOverflow = false

	exp := buildExporterWithOpts(t, intake.URL, tt,
		configoptional.Some(queueCfg),
		configretry.BackOffConfig{Enabled: false})

	// Flood the exporter: the worker is blocked so the queue saturates and
	// later submissions are rejected and counted by OTel.
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_ = exp.ConsumeMetrics(t.Context(), singleGaugeMetrics())
		})
	}
	wg.Wait()

	require.Eventually(t, func() bool {
		return otelCounterSum(t, tt, "otelcol_exporter_enqueue_failed_metric_points") > 0
	}, 5*time.Second, 50*time.Millisecond,
		"otelcol_exporter_enqueue_failed_metric_points should be positive when the OTel queue is full")
}

// TestDefaultForwarder_QueueFull_NoEnqueueFailedCounter documents that with
// the legacy async forwarder the otelcol_exporter_enqueue_failed_metric_points
// counter stays zero even when the underlying HTTP server is blocked. Because
// each OTel queue worker returns from ConsumeMetrics immediately (the async
// forwarder enqueues internally without waiting for the HTTP round-trip), the
// OTel queue drains at memory speed and never overflows. Drops that do occur
// are silently absorbed by the DefaultForwarder's own internal queue and are
// invisible to OTel operators.
func TestDefaultForwarder_QueueFull_NoEnqueueFailedCounter(t *testing.T) {
	setSyncForwarderGate(t, false)

	serverUnblock := make(chan struct{})
	intake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		_ = r.Body.Close()
		select {
		case <-serverUnblock:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer intake.Close()
	defer close(serverUnblock)

	tt := componenttest.NewTelemetry()
	t.Cleanup(func() { _ = tt.Shutdown(t.Context()) })

	// Use a queue large enough that 50 sends cannot overflow it; the point is
	// that the OTel queue worker returns instantly (async forwarder returns nil
	// before the HTTP round-trip completes) so the queue never saturates.
	queueCfg := exporterhelper.NewDefaultQueueConfig()
	queueCfg.QueueSize = 100
	queueCfg.NumConsumers = 1
	queueCfg.BlockOnOverflow = false

	exp := buildExporterWithOpts(t, intake.URL, tt,
		configoptional.Some(queueCfg),
		configretry.BackOffConfig{Enabled: false})

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_ = exp.ConsumeMetrics(t.Context(), singleGaugeMetrics())
		})
	}
	wg.Wait()

	assert.Never(t, func() bool {
		return otelCounterSum(t, tt, "otelcol_exporter_enqueue_failed_metric_points") > 0
	}, 200*time.Millisecond, 20*time.Millisecond,
		"async forwarder should not increment otelcol_exporter_enqueue_failed_metric_points")
}

// TestSyncForwarder_SendFailedMetricPointsCounter verifies that when
// UseSyncForwarder is enabled, a 4xx (permanent) response from the intake
// causes ConsumeMetrics to return a permanent error, which exporterhelper
// records as otelcol_exporter_send_failed_metric_points. This metric allows
// operators to detect actual data loss in dashboards and alerts.
func TestSyncForwarder_SendFailedMetricPointsCounter(t *testing.T) {
	setSyncForwarderGate(t, true)

	intake := newSyncForwarderIntake(http.StatusBadRequest)
	defer intake.Close()

	tt := componenttest.NewTelemetry()
	t.Cleanup(func() { _ = tt.Shutdown(t.Context()) })

	exp := buildExporterWithOpts(t, intake.URL, tt,
		configoptional.None[exporterhelper.QueueBatchConfig](),
		configretry.BackOffConfig{Enabled: false})

	err := exp.ConsumeMetrics(t.Context(), singleGaugeMetrics())
	require.Error(t, err, "sync forwarder should propagate the 400 error")
	assert.True(t, consumererror.IsPermanent(err), "400 response should be a permanent error")

	assert.Positive(t,
		otelCounterSum(t, tt, "otelcol_exporter_send_failed_metric_points"),
		"otelcol_exporter_send_failed_metric_points should be incremented on permanent send failure")
}

// TestDefaultForwarder_SendFailedMetricPointsCounter_NotIncremented documents
// that with the legacy async forwarder, otelcol_exporter_send_failed_metric_points
// is never incremented — even when the Datadog intake rejects every request —
// because ConsumeMetrics returns nil before the HTTP response is received.
func TestDefaultForwarder_SendFailedMetricPointsCounter_NotIncremented(t *testing.T) {
	setSyncForwarderGate(t, false)

	intake := newSyncForwarderIntake(http.StatusBadRequest)
	defer intake.Close()

	tt := componenttest.NewTelemetry()
	t.Cleanup(func() { _ = tt.Shutdown(t.Context()) })

	exp := buildExporterWithOpts(t, intake.URL, tt,
		configoptional.None[exporterhelper.QueueBatchConfig](),
		configretry.BackOffConfig{Enabled: false})

	err := exp.ConsumeMetrics(t.Context(), singleGaugeMetrics())
	assert.NoError(t, err, "async forwarder returns nil regardless of intake response")

	assert.Zero(t,
		otelCounterSum(t, tt, "otelcol_exporter_send_failed_metric_points"),
		"async forwarder should not increment otelcol_exporter_send_failed_metric_points")
}
