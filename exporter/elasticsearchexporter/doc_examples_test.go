// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

var update = flag.Bool("update", false, "update golden files in testdata/docs/")

const goldenDir = "testdata/docs"

func TestDocumentExamples(t *testing.T) {
	tStart := time.Date(2024, 3, 12, 20, 0, 41, 123456789, time.UTC)
	tEnd := time.Date(2024, 3, 12, 20, 0, 42, 123456789, time.UTC)

	type testCase struct {
		mode   MappingMode
		signal string
		encode func(t *testing.T, encoder documentEncoder) string
	}

	buildLog := func(t *testing.T, encoder documentEncoder) string {
		t.Helper()
		logs := plog.NewLogs()
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		resource := resourceLogs.Resource()
		resource.Attributes().PutStr("service.name", "my-service")
		resource.Attributes().PutStr("host.name", "my-host")
		resource.Attributes().PutStr("deployment.environment", "production")

		scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
		scopeLogs.Scope().SetName("io.opentelemetry.example")
		scopeLogs.Scope().SetVersion("1.0.0")

		record := scopeLogs.LogRecords().AppendEmpty()
		record.SetTimestamp(pcommon.NewTimestampFromTime(tStart))
		record.SetObservedTimestamp(pcommon.NewTimestampFromTime(tStart))
		record.SetSeverityNumber(plog.SeverityNumberError)
		record.SetSeverityText("ERROR")
		record.Body().SetStr("User authentication failed")
		record.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		record.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
		record.Attributes().PutStr("user.id", "user-123")
		record.Attributes().PutStr("event.name", "authentication.failure")

		var buf bytes.Buffer
		err := encoder.encodeLog(
			encodingContext{resource: resource, scope: scopeLogs.Scope()},
			record, elasticsearch.Index{}, &buf,
		)
		require.NoError(t, err)
		return buf.String()
	}

	buildTrace := func(t *testing.T, encoder documentEncoder) string {
		t.Helper()
		traces := ptrace.NewTraces()
		resourceSpans := traces.ResourceSpans().AppendEmpty()
		resource := resourceSpans.Resource()
		resource.Attributes().PutStr("service.name", "my-service")
		resource.Attributes().PutStr("host.name", "my-host")
		resource.Attributes().PutStr("deployment.environment", "production")

		scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
		scopeSpans.Scope().SetName("io.opentelemetry.example")
		scopeSpans.Scope().SetVersion("1.0.0")

		span := scopeSpans.Spans().AppendEmpty()
		span.SetName("GET /api/users")
		span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
		span.SetParentSpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1})
		span.SetKind(ptrace.SpanKindServer)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(tStart))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(tEnd))
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.Status().SetMessage("success")
		span.Attributes().PutStr("http.method", "GET")
		span.Attributes().PutStr("http.url", "/api/users")
		span.Attributes().PutInt("http.status_code", 200)

		var buf bytes.Buffer
		err := encoder.encodeSpan(
			encodingContext{resource: resource, scope: scopeSpans.Scope()},
			span, elasticsearch.Index{}, &buf,
		)
		require.NoError(t, err)
		return buf.String()
	}

	buildMetric := func(t *testing.T, encoder documentEncoder) string {
		t.Helper()
		metrics := pmetric.NewMetrics()
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		resource := resourceMetrics.Resource()
		resource.Attributes().PutStr("service.name", "my-service")
		resource.Attributes().PutStr("host.name", "my-host")

		scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
		scopeMetrics.Scope().SetName("io.opentelemetry.example")
		scopeMetrics.Scope().SetVersion("1.0.0")

		metric := scopeMetrics.Metrics().AppendEmpty()
		metric.SetName("http.server.request.duration")
		metric.SetUnit("s")
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(tStart))
		dp.SetDoubleValue(0.125)
		dp.Attributes().PutStr("http.method", "GET")

		dataPoint := datapoints.NewNumber(metric, dp)

		var buf bytes.Buffer
		var validationErrors []error
		_, err := encoder.encodeMetrics(
			encodingContext{resource: resource, scope: scopeMetrics.Scope()},
			[]datapoints.DataPoint{dataPoint}, &validationErrors, elasticsearch.Index{}, &buf,
		)
		require.NoError(t, err)
		require.Empty(t, validationErrors)
		return buf.String()
	}

	buildBodymapLog := func(t *testing.T, encoder documentEncoder) string {
		t.Helper()
		logs := plog.NewLogs()
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

		record := scopeLogs.LogRecords().AppendEmpty()
		record.SetObservedTimestamp(pcommon.NewTimestampFromTime(tStart))

		bodyMap := pcommon.NewMap()
		bodyMap.PutStr("@timestamp", "2024-03-12T20:00:41.123456789Z")
		bodyMap.PutStr("message", "User authentication failed")
		bodyMap.PutStr("level", "error")
		bodyMap.PutStr("service", "my-service")
		bodyMap.PutInt("status_code", 401)
		bodyMap.CopyTo(record.Body().SetEmptyMap())

		var buf bytes.Buffer
		err := encoder.encodeLog(
			encodingContext{resource: resourceLogs.Resource(), scope: scopeLogs.Scope()},
			record, elasticsearch.Index{}, &buf,
		)
		require.NoError(t, err)
		return buf.String()
	}

	tests := []testCase{
		// OTel mode
		{mode: MappingOTel, signal: "log", encode: buildLog},
		{mode: MappingOTel, signal: "trace", encode: buildTrace},
		{mode: MappingOTel, signal: "metric", encode: buildMetric},
		// ECS mode
		{mode: MappingECS, signal: "log", encode: buildLog},
		{mode: MappingECS, signal: "trace", encode: buildTrace},
		{mode: MappingECS, signal: "metric", encode: buildMetric},
		// None mode
		{mode: MappingNone, signal: "log", encode: buildLog},
		{mode: MappingNone, signal: "trace", encode: buildTrace},
		// Raw mode
		{mode: MappingRaw, signal: "log", encode: buildLog},
		{mode: MappingRaw, signal: "trace", encode: buildTrace},
		// Bodymap mode
		{mode: MappingBodyMap, signal: "log", encode: buildBodymapLog},
	}

	for _, tt := range tests {
		name := tt.mode.String() + "_" + tt.signal
		t.Run(name, func(t *testing.T) {
			encoder, err := newEncoder(tt.mode)
			require.NoError(t, err)

			raw := tt.encode(t, encoder)

			// Pretty-print JSON
			var pretty bytes.Buffer
			err = json.Indent(&pretty, []byte(raw), "", "  ")
			require.NoError(t, err)
			prettyJSON := pretty.String() + "\n"

			goldenFile := filepath.Join(goldenDir, name+".json")

			if *update {
				err = os.WriteFile(goldenFile, []byte(prettyJSON), 0600)
				require.NoError(t, err)
				return
			}

			expected, err := os.ReadFile(goldenFile)
			require.NoError(t, err, "golden file %s not found, run with -update to generate", goldenFile)
			assert.JSONEq(t, string(expected), prettyJSON)
		})
	}
}
