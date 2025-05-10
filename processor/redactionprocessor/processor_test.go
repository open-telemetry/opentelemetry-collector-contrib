// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
)

// TestRedactUnknownAttributes validates that the processor deletes span
// attributes that are not the allowed keys list
func TestRedactUnknownAttributes(t *testing.T) {
	config := &Config{
		AllowedKeys: []string{"group", "id", "name"},
	}
	allowed := map[string]pcommon.Value{
		"group": pcommon.NewValueStr("temporary"),
		"id":    pcommon.NewValueInt(5),
		"name":  pcommon.NewValueStr("placeholder"),
	}
	ignored := map[string]pcommon.Value{
		"safe_attribute": pcommon.NewValueStr("4111111111111112"),
	}
	redacted := map[string]pcommon.Value{
		"credit_card": pcommon.NewValueStr("4111111111111111"),
	}

	outTraces := runTest(t, allowed, redacted, nil, ignored, config)
	outLogs := runLogsTest(t, allowed, redacted, nil, ignored, config)
	outMetricsGauge := runMetricsTest(t, allowed, redacted, nil, ignored, config, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, allowed, redacted, nil, ignored, config, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, allowed, redacted, nil, ignored, config, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, allowed, redacted, nil, ignored, config, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, allowed, redacted, nil, ignored, config, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		for k, v := range allowed {
			val, ok := attr.Get(k)
			assert.True(t, ok)
			assert.Equal(t, v.AsRaw(), val.AsRaw())
		}
		for k := range redacted {
			_, ok := attr.Get(k)
			assert.False(t, ok)
		}
	}
}

// TestAllowAllKeys validates that the processor does not delete
// span attributes that are not the allowed keys list if Config.AllowAllKeys
// is set to true
func TestAllowAllKeys(t *testing.T) {
	config := &Config{
		AllowedKeys:  []string{"group", "id"},
		AllowAllKeys: true,
	}
	allowed := map[string]pcommon.Value{
		"group": pcommon.NewValueStr("temporary"),
		"id":    pcommon.NewValueInt(5),
		"name":  pcommon.NewValueStr("placeholder"),
	}

	outTraces := runTest(t, allowed, nil, nil, nil, config)
	outLogs := runLogsTest(t, allowed, nil, nil, nil, config)
	outMetricsGauge := runMetricsTest(t, allowed, nil, nil, nil, config, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, allowed, nil, nil, nil, config, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, allowed, nil, nil, nil, config, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, allowed, nil, nil, nil, config, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, allowed, nil, nil, nil, config, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		for k, v := range allowed {
			val, ok := attr.Get(k)
			assert.True(t, ok)
			assert.Equal(t, v.AsRaw(), val.AsRaw())
		}
		value, _ := attr.Get("name")
		assert.Equal(t, "placeholder", value.Str())
	}
}

// TestAllowAllKeysMaskValues validates that the processor still redacts
// span attribute values if Config.AllowAllKeys is set to true
func TestAllowAllKeysMaskValues(t *testing.T) {
	config := &Config{
		AllowedKeys:   []string{"group", "id", "name"},
		BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
		AllowAllKeys:  true,
	}
	allowed := map[string]pcommon.Value{
		"group": pcommon.NewValueStr("temporary"),
		"id":    pcommon.NewValueInt(5),
		"name":  pcommon.NewValueStr("placeholder"),
	}
	masked := map[string]pcommon.Value{
		"credit_card": pcommon.NewValueStr("placeholder 4111111111111111"),
	}

	outTraces := runTest(t, allowed, nil, masked, nil, config)
	outLogs := runLogsTest(t, allowed, nil, masked, nil, config)
	outMetricsGauge := runMetricsTest(t, allowed, nil, masked, nil, config, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, allowed, nil, masked, nil, config, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, allowed, nil, masked, nil, config, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, allowed, nil, masked, nil, config, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, allowed, nil, masked, nil, config, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		for k, v := range allowed {
			val, ok := attr.Get(k)
			assert.True(t, ok)
			assert.Equal(t, v.AsRaw(), val.AsRaw())
		}
		value, _ := attr.Get("credit_card")
		assert.Equal(t, "placeholder ****", value.Str())
	}
}

// TODO: Test redaction with metric tags in a metrics PR

// TestRedactSummaryDebug validates that the processor writes a verbose summary
// of any attributes it deleted to the new redaction.redacted.keys and
// redaction.redacted.count span attributes while set to full debug output
func TestRedactSummaryDebug(t *testing.T) {
	config := &Config{
		AllowedKeys:   []string{"id", "group", "name", "group.id", "member (id)"},
		BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
		IgnoredKeys:   []string{"safe_attribute"},
		Summary:       "debug",
	}
	allowed := map[string]pcommon.Value{
		"id":          pcommon.NewValueInt(5),
		"group.id":    pcommon.NewValueStr("some.valid.id"),
		"member (id)": pcommon.NewValueStr("some other valid id"),
	}
	masked := map[string]pcommon.Value{
		"name": pcommon.NewValueStr("placeholder 4111111111111111"),
	}
	ignored := map[string]pcommon.Value{
		"safe_attribute": pcommon.NewValueStr("harmless 4111111111111112"),
	}
	redacted := map[string]pcommon.Value{
		"credit_card": pcommon.NewValueStr("4111111111111111"),
	}

	outTraces := runTest(t, allowed, redacted, masked, ignored, config)
	outLogs := runLogsTest(t, allowed, redacted, masked, ignored, config)
	outMetricsGauge := runMetricsTest(t, allowed, redacted, masked, ignored, config, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, allowed, redacted, masked, ignored, config, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, allowed, redacted, masked, ignored, config, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, allowed, redacted, masked, ignored, config, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, allowed, redacted, masked, ignored, config, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		deleted := make([]string, 0, len(redacted))
		for k := range redacted {
			_, ok := attr.Get(k)
			assert.False(t, ok)
			deleted = append(deleted, k)
		}
		maskedKeys, ok := attr.Get(redactedKeys)
		assert.True(t, ok)
		sort.Strings(deleted)
		assert.Equal(t, strings.Join(deleted, ","), maskedKeys.Str())
		maskedKeyCount, ok := attr.Get(redactedKeyCount)
		assert.True(t, ok)
		assert.Equal(t, int64(len(deleted)), maskedKeyCount.Int())

		ignoredKeyCount, ok := attr.Get(ignoredKeyCount)
		assert.True(t, ok)
		assert.Equal(t, int64(len(ignored)), ignoredKeyCount.Int())

		blockedKeys := []string{"name"}
		maskedValues, ok := attr.Get(maskedValues)
		assert.True(t, ok)
		assert.Equal(t, strings.Join(blockedKeys, ","), maskedValues.Str())
		maskedValueCount, ok := attr.Get(maskedValueCount)
		assert.True(t, ok)
		assert.Equal(t, int64(1), maskedValueCount.Int())
		value, _ := attr.Get("name")
		assert.Equal(t, "placeholder ****", value.Str())
	}
}

// TestRedactSummaryInfo validates that the processor writes a verbose summary
// of any attributes it deleted to the new redaction.redacted.count span
// attribute (but not to redaction.redacted.keys) when set to the info level
// of output
func TestRedactSummaryInfo(t *testing.T) {
	config := &Config{
		AllowedKeys:   []string{"id", "name", "group"},
		BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
		IgnoredKeys:   []string{"safe_attribute"},
		Summary:       "info"}
	allowed := map[string]pcommon.Value{
		"id": pcommon.NewValueInt(5),
	}
	ignored := map[string]pcommon.Value{
		"safe_attribute": pcommon.NewValueStr("harmless but suspicious 4111111111111141"),
	}
	masked := map[string]pcommon.Value{
		"name": pcommon.NewValueStr("placeholder 4111111111111111"),
	}
	redacted := map[string]pcommon.Value{
		"credit_card": pcommon.NewValueStr("4111111111111111"),
	}

	outTraces := runTest(t, allowed, redacted, masked, ignored, config)
	outLogs := runLogsTest(t, allowed, redacted, masked, ignored, config)
	outMetricsGauge := runMetricsTest(t, allowed, redacted, masked, ignored, config, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, allowed, redacted, masked, ignored, config, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, allowed, redacted, masked, ignored, config, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, allowed, redacted, masked, ignored, config, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, allowed, redacted, masked, ignored, config, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		deleted := make([]string, 0, len(redacted))
		for k := range redacted {
			_, ok := attr.Get(k)
			assert.False(t, ok)
			deleted = append(deleted, k)
		}
		_, ok := attr.Get(redactedKeys)
		assert.False(t, ok)
		maskedKeyCount, ok := attr.Get(redactedKeyCount)
		assert.True(t, ok)
		assert.Equal(t, int64(len(deleted)), maskedKeyCount.Int())
		_, ok = attr.Get(maskedValues)
		assert.False(t, ok)

		maskedValueCount, ok := attr.Get(maskedValueCount)
		assert.True(t, ok)
		assert.Equal(t, int64(1), maskedValueCount.Int())
		value, _ := attr.Get("name")
		assert.Equal(t, "placeholder ****", value.Str())

		ignoredKeyCount, ok := attr.Get(ignoredKeyCount)
		assert.True(t, ok)
		assert.Equal(t, int64(1), ignoredKeyCount.Int())
		value, _ = attr.Get("safe_attribute")
		assert.Equal(t, "harmless but suspicious 4111111111111141", value.Str())
	}
}

// TestRedactSummarySilent validates that the processor does not create the
// summary attributes when set to silent
func TestRedactSummarySilent(t *testing.T) {
	config := &Config{AllowedKeys: []string{"id", "name", "group"},
		BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
		Summary:       "silent"}
	allowed := map[string]pcommon.Value{
		"id": pcommon.NewValueInt(5),
	}
	masked := map[string]pcommon.Value{
		"name": pcommon.NewValueStr("placeholder 4111111111111111"),
	}
	redacted := map[string]pcommon.Value{
		"credit_card": pcommon.NewValueStr("4111111111111111"),
	}

	outTraces := runTest(t, allowed, redacted, masked, nil, config)
	outLogs := runLogsTest(t, allowed, redacted, masked, nil, config)
	outMetricsGauge := runMetricsTest(t, allowed, redacted, masked, nil, config, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, allowed, redacted, masked, nil, config, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, allowed, redacted, masked, nil, config, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, allowed, redacted, masked, nil, config, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, allowed, redacted, masked, nil, config, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		for k := range redacted {
			_, ok := attr.Get(k)
			assert.False(t, ok)
		}
		_, ok := attr.Get(redactedKeys)
		assert.False(t, ok)
		_, ok = attr.Get(redactedKeyCount)
		assert.False(t, ok)
		_, ok = attr.Get(maskedValues)
		assert.False(t, ok)
		_, ok = attr.Get(maskedValueCount)
		assert.False(t, ok)
		value, _ := attr.Get("name")
		assert.Equal(t, "placeholder ****", value.Str())
	}
}

// TestRedactSummaryDefault validates that the processor does not create the
// summary attributes by default
func TestRedactSummaryDefault(t *testing.T) {
	config := &Config{AllowedKeys: []string{"id", "name", "group"}}
	allowed := map[string]pcommon.Value{
		"id": pcommon.NewValueInt(5),
	}
	ignored := map[string]pcommon.Value{
		"internal": pcommon.NewValueStr("a harmless 12 digits 4111111111111113"),
	}
	masked := map[string]pcommon.Value{
		"name": pcommon.NewValueStr("placeholder 4111111111111111"),
	}

	outTraces := runTest(t, allowed, nil, masked, ignored, config)
	outLogs := runLogsTest(t, allowed, nil, masked, ignored, config)
	outMetricsGauge := runMetricsTest(t, allowed, nil, masked, ignored, config, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, allowed, nil, masked, ignored, config, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, allowed, nil, masked, ignored, config, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, allowed, nil, masked, ignored, config, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, allowed, nil, masked, ignored, config, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		_, ok := attr.Get(redactedKeys)
		assert.False(t, ok)
		_, ok = attr.Get(redactedKeyCount)
		assert.False(t, ok)
		_, ok = attr.Get(maskedValues)
		assert.False(t, ok)
		_, ok = attr.Get(maskedValueCount)
		assert.False(t, ok)
		_, ok = attr.Get(ignoredKeyCount)
		assert.False(t, ok)
	}
}

// TestMultipleBlockValues validates that the processor can block multiple
// patterns
func TestMultipleBlockValues(t *testing.T) {
	config := &Config{AllowedKeys: []string{"id", "name", "mystery"},
		BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?", "(5[1-5][0-9]{3})"},
		Summary:       "debug"}
	allowed := map[string]pcommon.Value{
		"id":      pcommon.NewValueInt(5),
		"mystery": pcommon.NewValueStr("mystery 52000"),
	}
	masked := map[string]pcommon.Value{
		"name": pcommon.NewValueStr("placeholder 4111111111111111 52000"),
	}
	redacted := map[string]pcommon.Value{
		"credit_card": pcommon.NewValueStr("4111111111111111"),
	}

	outTraces := runTest(t, allowed, redacted, masked, nil, config)
	outLogs := runLogsTest(t, allowed, redacted, masked, nil, config)
	outMetricsGauge := runMetricsTest(t, allowed, redacted, masked, nil, config, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, allowed, redacted, masked, nil, config, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, allowed, redacted, masked, nil, config, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, allowed, redacted, masked, nil, config, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, allowed, redacted, masked, nil, config, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		deleted := make([]string, 0, len(redacted))
		for k := range redacted {
			_, ok := attr.Get(k)
			assert.False(t, ok)
			deleted = append(deleted, k)
		}
		maskedKeys, ok := attr.Get(redactedKeys)
		assert.True(t, ok)
		assert.Equal(t, strings.Join(deleted, ","), maskedKeys.Str())
		maskedKeyCount, ok := attr.Get(redactedKeyCount)
		assert.True(t, ok)
		assert.Equal(t, int64(len(deleted)), maskedKeyCount.Int())

		blockedKeys := []string{"name", "mystery"}
		maskedValues, ok := attr.Get(maskedValues)
		assert.True(t, ok)
		sort.Strings(blockedKeys)
		assert.Equal(t, strings.Join(blockedKeys, ","), maskedValues.Str())
		assert.Equal(t, pcommon.ValueTypeStr, maskedValues.Type())
		assert.Equal(t, strings.Join(blockedKeys, ","), maskedValues.Str())
		maskedValueCount, ok := attr.Get(maskedValueCount)
		assert.True(t, ok)
		assert.Equal(t, int64(len(blockedKeys)), maskedValueCount.Int())
		nameValue, _ := attr.Get("name")
		mysteryValue, _ := attr.Get("mystery")
		assert.Equal(t, "placeholder **** ****", nameValue.Str())
		assert.Equal(t, "mystery ****", mysteryValue.Str())
	}
}

// TestProcessAttrsAppliedTwice validates a use case when data is coming through redaction processor more than once.
// Existing attributes must be updated, not overridden or ignored.
func TestProcessAttrsAppliedTwice(t *testing.T) {
	config := &Config{
		AllowedKeys:   []string{"id", "credit_card", "mystery"},
		BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
		Summary:       "debug",
	}
	processor, err := newRedaction(context.TODO(), config, zaptest.NewLogger(t))
	require.NoError(t, err)

	attrs := pcommon.NewMap()
	assert.NoError(t, attrs.FromRaw(map[string]any{
		"id":             5,
		"redundant":      1.2,
		"mystery":        "mystery ****",
		"credit_card":    "4111111111111111",
		redactedKeys:     "dropped_attr1,dropped_attr2",
		redactedKeyCount: 2,
		maskedValues:     "mystery",
		maskedValueCount: 1,
	}))
	processor.processAttrs(context.TODO(), attrs)

	assert.Equal(t, 7, attrs.Len())
	val, found := attrs.Get(redactedKeys)
	assert.True(t, found)
	assert.Equal(t, "dropped_attr1,dropped_attr2,redundant", val.Str())
	val, found = attrs.Get(redactedKeyCount)
	assert.True(t, found)
	assert.Equal(t, int64(3), val.Int())
	val, found = attrs.Get(maskedValues)
	assert.True(t, found)
	assert.Equal(t, "credit_card,mystery", val.Str())
	val, found = attrs.Get(maskedValueCount)
	assert.True(t, found)
	assert.Equal(t, int64(2), val.Int())
}

// runTest transforms the test input data and passes it through the processor
func runTest(
	t *testing.T,
	allowed map[string]pcommon.Value,
	redacted map[string]pcommon.Value,
	masked map[string]pcommon.Value,
	ignored map[string]pcommon.Value,
	config *Config,
) ptrace.Traces {
	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	library := ils.Scope()
	library.SetName("first-library")
	span := ils.Spans().AppendEmpty()
	span.SetName("first-batch-first-span")
	span.SetTraceID([16]byte{1, 2, 3, 4})

	length := len(allowed) + len(masked) + len(redacted) + len(ignored)
	for k, v := range allowed {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}
	for k, v := range masked {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}
	for k, v := range redacted {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}
	for k, v := range ignored {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}

	assert.Equal(t, span.Attributes().Len(), length)
	assert.Equal(t, ils.Spans().At(0).Attributes().Len(), length)
	assert.Equal(t, inBatch.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Len(), length)

	// test
	ctx := context.Background()
	processor, err := newRedaction(ctx, config, zaptest.NewLogger(t))
	assert.NoError(t, err)
	outBatch, err := processor.processTraces(ctx, inBatch)

	// verify
	assert.NoError(t, err)
	return outBatch
}

// runLogsTest transforms the test input log data and passes it through the processor
func runLogsTest(
	t *testing.T,
	allowed map[string]pcommon.Value,
	redacted map[string]pcommon.Value,
	masked map[string]pcommon.Value,
	ignored map[string]pcommon.Value,
	config *Config,
) plog.Logs {
	inBatch := plog.NewLogs()
	rl := inBatch.ResourceLogs().AppendEmpty()
	ils := rl.ScopeLogs().AppendEmpty()
	_ = rl.ScopeLogs().AppendEmpty()

	library := ils.Scope()
	library.SetName("first-library")
	logEntry := ils.LogRecords().AppendEmpty()
	logEntry.Body().SetStr("first-batch-first-logEntry")
	logEntry.SetTraceID([16]byte{1, 2, 3, 4})

	length := len(allowed) + len(masked) + len(redacted) + len(ignored)
	for k, v := range allowed {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
	}
	for k, v := range masked {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
	}
	for k, v := range redacted {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
	}
	for k, v := range ignored {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
	}

	assert.Equal(t, logEntry.Attributes().Len(), length)
	assert.Equal(t, ils.LogRecords().At(0).Attributes().Len(), length)
	assert.Equal(t, inBatch.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Len(), length)

	// test
	ctx := context.Background()
	processor, err := newRedaction(ctx, config, zaptest.NewLogger(t))
	assert.NoError(t, err)
	outBatch, err := processor.processLogs(ctx, inBatch)

	// verify
	assert.NoError(t, err)
	return outBatch
}

// runMetricsTest transforms the test input metric data and passes it through the processor
func runMetricsTest(
	t *testing.T,
	allowed map[string]pcommon.Value,
	redacted map[string]pcommon.Value,
	masked map[string]pcommon.Value,
	ignored map[string]pcommon.Value,
	config *Config,
	metricType pmetric.MetricType,
) pmetric.Metrics {
	inBatch := pmetric.NewMetrics()
	rl := inBatch.ResourceMetrics().AppendEmpty()
	ils := rl.ScopeMetrics().AppendEmpty()

	library := ils.Scope()
	library.SetName("first-library")
	metric := ils.Metrics().AppendEmpty()
	metric.SetDescription("first-batch-first-metric")

	length := len(allowed) + len(masked) + len(redacted) + len(ignored)

	var dataPointAttrs pcommon.Map
	switch metricType {
	case pmetric.MetricTypeGauge:
		dataPointAttrs = metric.SetEmptyGauge().DataPoints().AppendEmpty().Attributes()
	case pmetric.MetricTypeSum:
		dataPointAttrs = metric.SetEmptySum().DataPoints().AppendEmpty().Attributes()
	case pmetric.MetricTypeHistogram:
		dataPointAttrs = metric.SetEmptyHistogram().DataPoints().AppendEmpty().Attributes()
	case pmetric.MetricTypeExponentialHistogram:
		dataPointAttrs = metric.SetEmptyExponentialHistogram().DataPoints().AppendEmpty().Attributes()
	case pmetric.MetricTypeSummary:
		dataPointAttrs = metric.SetEmptySummary().DataPoints().AppendEmpty().Attributes()
	case pmetric.MetricTypeEmpty:
	}
	for k, v := range allowed {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
	}
	for k, v := range masked {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
	}
	for k, v := range redacted {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
	}
	for k, v := range ignored {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
	}

	assert.Equal(t, length, dataPointAttrs.Len())
	assert.Equal(t, length, rl.Resource().Attributes().Len())

	// test
	ctx := context.Background()
	processor, err := newRedaction(ctx, config, zaptest.NewLogger(t))
	assert.NoError(t, err)
	outBatch, err := processor.processMetrics(ctx, inBatch)

	// verify
	assert.NoError(t, err)
	return outBatch
}

// BenchmarkRedactSummaryDebug measures the performance impact of running the processor
// with full debug level of output for redacting span attributes not on the allowed
// keys list
func BenchmarkRedactSummaryDebug(b *testing.B) {
	config := &Config{
		AllowedKeys:   []string{"id", "group", "name", "group.id", "member (id)"},
		BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
		IgnoredKeys:   []string{"safe_attribute"},
		Summary:       "debug",
	}
	allowed := map[string]pcommon.Value{
		"id":          pcommon.NewValueInt(5),
		"group.id":    pcommon.NewValueStr("some.valid.id"),
		"member (id)": pcommon.NewValueStr("some other valid id"),
	}
	masked := map[string]pcommon.Value{
		"name": pcommon.NewValueStr("placeholder 4111111111111111"),
	}
	ignored := map[string]pcommon.Value{
		"safe_attribute": pcommon.NewValueStr("harmless 4111111111111112"),
	}
	redacted := map[string]pcommon.Value{
		"credit_card": pcommon.NewValueStr("would be nice"),
	}
	ctx := context.Background()
	processor, _ := newRedaction(ctx, config, zaptest.NewLogger(b))

	for i := 0; i < b.N; i++ {
		runBenchmark(allowed, redacted, masked, ignored, processor)
	}
}

// BenchmarkMaskSummaryDebug measures the performance impact of running the processor
// with full debug level of output for masking span attribute values on the
// blocked values list
func BenchmarkMaskSummaryDebug(b *testing.B) {
	config := &Config{
		AllowedKeys:   []string{"id", "group", "name", "url"},
		BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?", "(http|https|ftp):[\\/]{2}([a-zA-Z0-9\\-\\.]+\\.[a-zA-Z]{2,4})(:[0-9]+)?\\/?([a-zA-Z0-9\\-\\._\\?\\,\\'\\/\\\\\\+&amp;%\\$#\\=~]*)"},
		IgnoredKeys:   []string{"safe_attribute", "also_safe"},
		Summary:       "debug",
	}
	allowed := map[string]pcommon.Value{
		"id":          pcommon.NewValueInt(5),
		"group.id":    pcommon.NewValueStr("some.valid.id"),
		"member (id)": pcommon.NewValueStr("some other valid id"),
	}
	masked := map[string]pcommon.Value{
		"name": pcommon.NewValueStr("placeholder 4111111111111111"),
		"url":  pcommon.NewValueStr("https://www.this_is_testing_url.com"),
	}
	ignored := map[string]pcommon.Value{
		"safe_attribute": pcommon.NewValueStr("suspicious 4111111111111112"),
		"also_safe":      pcommon.NewValueStr("suspicious 4111111111111113"),
	}
	ctx := context.Background()
	processor, _ := newRedaction(ctx, config, zaptest.NewLogger(b))

	for i := 0; i < b.N; i++ {
		runBenchmark(allowed, nil, masked, ignored, processor)
	}
}

// runBenchmark transform benchmark input and runs it through the processor
func runBenchmark(
	allowed map[string]pcommon.Value,
	redacted map[string]pcommon.Value,
	masked map[string]pcommon.Value,
	ignored map[string]pcommon.Value,
	processor *redaction,
) {
	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	library := ils.Scope()
	library.SetName("first-library")
	span := ils.Spans().AppendEmpty()
	span.SetName("first-batch-first-span")
	span.SetTraceID([16]byte{1, 2, 3, 4})

	for k, v := range allowed {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}
	for k, v := range masked {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}
	for k, v := range redacted {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}
	for k, v := range ignored {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}

	_, _ = processor.processTraces(context.Background(), inBatch)
}
