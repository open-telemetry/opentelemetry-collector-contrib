// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package semconvtest_test

import (
	"encoding/json"
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

type findingContext struct {
	AttributeName string `json:"attribute_name,omitempty"`
}

func getAttributeName(f semconvtest.PolicyFinding) string {
	var ctx findingContext
	if err := json.Unmarshal(f.Context, &ctx); err != nil {
		return ""
	}
	return ctx.AttributeName
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
	opts := &semconvtest.WeaverOptions{}

	weaver, err := semconvtest.NewWeaverContext(t.Context(), opts)
	require.NoError(t, err)
	defer func() { require.NoError(t, weaver.Shutdown()) }()

	logs := plog.NewLogs()
	res := logs.ResourceLogs().AppendEmpty()
	res.Resource().Attributes().PutStr("invalid.resource.attribute", "value")
	scope := res.ScopeLogs().AppendEmpty()
	record := scope.LogRecords().AppendEmpty()
	record.Body().SetStr("hi I am a log")

	err = weaver.TestLogs(logs)
	require.NoError(t, err)

	content, err := weaver.Stop()
	require.NoError(t, err)

	report, err := semconvtest.ParseLiveCheckReport(content)
	require.NoError(t, err)

	require.NotEmpty(t, report.Samples)

	require.True(t, report.HasViolations(), "expected violations for unknown attribute")

	violations := report.GetViolations()

	require.Len(t, violations, 1, "expected 1 violation: resource attr")

	resourceAttrViolation := findViolationByAttributeName(violations, "invalid.resource.attribute")
	require.NotNil(t, resourceAttrViolation, "expected violation for invalid.resource.attribute")
	require.Equal(t, semconvtest.FindingLevelViolation, resourceAttrViolation.Level)
}

func TestWeaverMetrics(t *testing.T) {
	opts := &semconvtest.WeaverOptions{}

	weaver, err := semconvtest.NewWeaverContext(t.Context(), opts)
	require.NoError(t, err)
	defer func() { require.NoError(t, weaver.Shutdown()) }()

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

	err = weaver.TestMetrics(metrics)
	require.NoError(t, err)

	content, err := weaver.Stop()
	require.NoError(t, err)

	report, err := semconvtest.ParseLiveCheckReport(content)
	require.NoError(t, err)

	require.NotEmpty(t, report.Samples)

	require.True(t, report.HasViolations(), "expected violations for invalid metric")

	violations := report.GetViolations()

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
	opts := &semconvtest.WeaverOptions{}

	weaver, err := semconvtest.NewWeaverContext(t.Context(), opts)
	require.NoError(t, err)
	defer func() { require.NoError(t, weaver.Shutdown()) }()

	traces := ptrace.NewTraces()
	res := traces.ResourceSpans().AppendEmpty()
	res.Resource().Attributes().PutStr("invalid.resource.attribute", "value")
	scope := res.ScopeSpans().AppendEmpty()
	span := scope.Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetKind(ptrace.SpanKindClient)
	span.Attributes().PutStr("invalid.span.attribute", "value")

	err = weaver.TestTraces(traces)
	require.NoError(t, err)

	content, err := weaver.Stop()
	require.NoError(t, err)

	report, err := semconvtest.ParseLiveCheckReport(content)
	require.NoError(t, err)

	require.NotEmpty(t, report.Samples)

	require.True(t, report.HasViolations(), "expected violations for invalid span attributes")

	violations := report.GetViolations()

	require.Len(t, violations, 2, "expected 2 violations: resource attr and span attr")

	resourceAttrViolation := findViolationByAttributeName(violations, "invalid.resource.attribute")
	require.NotNil(t, resourceAttrViolation, "expected violation for invalid.resource.attribute")
	require.Equal(t, semconvtest.FindingLevelViolation, resourceAttrViolation.Level)

	spanAttrViolation := findViolationByAttributeName(violations, "invalid.span.attribute")
	require.NotNil(t, spanAttrViolation, "expected violation for invalid.span.attribute (tests span traversal)")
	require.Equal(t, semconvtest.FindingLevelViolation, spanAttrViolation.Level)
}

// TestWeaverHTTPServerMetrics demonstrates how a receiver author would use
// semconvtest to validate their component's telemetry against semantic conventions.
// It creates a sample receiver that emits valid HTTP semconv metrics and asserts
// that Weaver reports no violations.
func TestWeaverHTTPServerMetrics(t *testing.T) {
	opts := &semconvtest.WeaverOptions{}

	weaver, err := semconvtest.NewWeaverContext(t.Context(), opts)
	require.NoError(t, err)
	defer func() { require.NoError(t, weaver.Shutdown()) }()

	factory := samplereceiver.NewFactory()
	sink := &consumertest.MetricsSink{}
	settings := receivertest.NewNopSettings(component.MustNewType("sample_http"))
	recv, err := factory.CreateMetrics(t.Context(), settings, factory.CreateDefaultConfig(), sink)
	require.NoError(t, err)

	err = recv.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() { require.NoError(t, recv.Shutdown(t.Context())) }()

	require.NotEmpty(t, sink.AllMetrics(), "expected receiver to produce metrics")
	metrics := sink.AllMetrics()[0]

	err = weaver.TestMetrics(metrics)
	require.NoError(t, err)

	content, err := weaver.Stop()
	require.NoError(t, err)

	report, err := semconvtest.ParseLiveCheckReport(content)
	require.NoError(t, err)

	require.NotEmpty(t, report.Samples)

	require.False(t, report.HasViolations(),
		"expected no violations for valid HTTP semconv metrics, got: %v", report.GetViolations())
}
