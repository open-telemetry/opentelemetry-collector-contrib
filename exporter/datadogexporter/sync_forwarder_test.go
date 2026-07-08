// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix

package datadogexporter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/serializerexporter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

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
