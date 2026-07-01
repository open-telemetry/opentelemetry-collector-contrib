// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package semconvtest_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest/internal/samplereceiver"
)

// recordingTB wraps a *testing.T and captures failures reported via Errorf,
// so tests can assert that semconvtest marks violations as test failures
// without failing the real test. Fatal infrastructure errors still pass
// through to the embedded testing.TB.
type recordingTB struct {
	testing.TB
	failed bool
	errors []string
}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.failed = true
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

// findingDetails holds the fields we care about from a finding's context
// payload.
type findingDetails struct {
	AttributeName string `json:"attribute_name,omitempty"`
}

func getAttributeName(f semconvtest.PolicyFinding) string {
	var details findingDetails
	if err := json.Unmarshal(f.Context, &details); err != nil {
		return ""
	}
	return details.AttributeName
}

func findViolationByAttributeName(violations []semconvtest.PolicyFinding, attrName string) *semconvtest.PolicyFinding {
	for i, v := range violations {
		if v.ID == "missing_attribute" && getAttributeName(v) == attrName {
			return &violations[i]
		}
	}
	return nil
}

func findViolationByID(violations []semconvtest.PolicyFinding, id string) *semconvtest.PolicyFinding {
	for i, v := range violations {
		if v.ID == id {
			return &violations[i]
		}
	}
	return nil
}

func TestWeaverLogs(t *testing.T) {
	logs := plog.NewLogs()
	res := logs.ResourceLogs().AppendEmpty()
	res.Resource().Attributes().PutStr("invalid.resource.attribute", "value")
	scope := res.ScopeLogs().AppendEmpty()
	record := scope.LogRecords().AppendEmpty()
	record.Body().SetStr("hi I am a log")

	rec := &recordingTB{TB: t}
	violations := semconvtest.TestLogs(rec, logs)

	require.True(t, rec.failed, "expected TestLogs to mark the test as failed")
	require.Len(t, violations, 1, "expected 1 violation: resource attr")

	resourceAttrViolation := findViolationByAttributeName(violations, "invalid.resource.attribute")
	require.NotNil(t, resourceAttrViolation, "expected violation for invalid.resource.attribute")
	require.Equal(t, semconvtest.FindingLevelViolation, resourceAttrViolation.Level)
}

func TestWeaverMetrics(t *testing.T) {
	metrics := pmetric.NewMetrics()
	res := metrics.ResourceMetrics().AppendEmpty()
	res.Resource().Attributes().PutStr("invalid.resource.attribute", "value")
	scope := res.ScopeMetrics().AppendEmpty()
	metric := scope.Metrics().AppendEmpty()
	metric.SetName("invalid.metric.name")
	metric.SetUnit("1")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetIntValue(42)
	dp.Attributes().PutStr("invalid.datapoint.attribute", "value")

	rec := &recordingTB{TB: t}
	violations := semconvtest.TestMetrics(rec, metrics)

	require.True(t, rec.failed, "expected TestMetrics to mark the test as failed")
	require.Len(t, violations, 3, "expected 3 violations: resource attr, metric, and data point attr")

	resourceAttrViolation := findViolationByAttributeName(violations, "invalid.resource.attribute")
	require.NotNil(t, resourceAttrViolation, "expected violation for invalid.resource.attribute")
	require.Equal(t, semconvtest.FindingLevelViolation, resourceAttrViolation.Level)

	metricViolation := findViolationByID(violations, "missing_metric")
	require.NotNil(t, metricViolation, "expected missing_metric violation")
	require.Equal(t, semconvtest.FindingLevelViolation, metricViolation.Level)

	dataPointAttrViolation := findViolationByAttributeName(violations, "invalid.datapoint.attribute")
	require.NotNil(t, dataPointAttrViolation, "expected violation for invalid.datapoint.attribute (tests data point traversal)")
	require.Equal(t, semconvtest.FindingLevelViolation, dataPointAttrViolation.Level)
}

func TestWeaverTraces(t *testing.T) {
	traces := ptrace.NewTraces()
	res := traces.ResourceSpans().AppendEmpty()
	res.Resource().Attributes().PutStr("invalid.resource.attribute", "value")
	scope := res.ScopeSpans().AppendEmpty()
	span := scope.Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetKind(ptrace.SpanKindClient)
	span.Attributes().PutStr("invalid.span.attribute", "value")

	rec := &recordingTB{TB: t}
	violations := semconvtest.TestTraces(rec, traces)

	require.True(t, rec.failed, "expected TestTraces to mark the test as failed")
	require.Len(t, violations, 2, "expected 2 violations: resource attr and span attr")

	resourceAttrViolation := findViolationByAttributeName(violations, "invalid.resource.attribute")
	require.NotNil(t, resourceAttrViolation, "expected violation for invalid.resource.attribute")
	require.Equal(t, semconvtest.FindingLevelViolation, resourceAttrViolation.Level)

	spanAttrViolation := findViolationByAttributeName(violations, "invalid.span.attribute")
	require.NotNil(t, spanAttrViolation, "expected violation for invalid.span.attribute (tests span traversal)")
	require.Equal(t, semconvtest.FindingLevelViolation, spanAttrViolation.Level)
}

// TestWeaverHTTPServerMetrics demonstrates how a receiver author would use
// semconvtest to validate their component's telemetry against semantic
// conventions: produce pdata with the component, then hand it to
// semconvtest.TestMetrics along with the *testing.T.
func TestWeaverHTTPServerMetrics(t *testing.T) {
	factory := samplereceiver.NewFactory()
	sink := &consumertest.MetricsSink{}
	settings := receivertest.NewNopSettings(component.MustNewType("sample_http"))
	recv, err := factory.CreateMetrics(t.Context(), settings, factory.CreateDefaultConfig(), sink)
	require.NoError(t, err)

	require.NoError(t, recv.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, recv.Shutdown(t.Context())) }()

	require.NotEmpty(t, sink.AllMetrics(), "expected receiver to produce metrics")

	semconvtest.TestMetrics(t, sink.AllMetrics()[0])
}
