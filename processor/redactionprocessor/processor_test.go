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

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/db"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/url"
)

type testConfig struct {
	allowed       map[string]pcommon.Value
	ignored       map[string]pcommon.Value
	redacted      map[string]pcommon.Value
	masked        map[string]pcommon.Value
	blockedKeys   map[string]pcommon.Value
	allowedValues map[string]pcommon.Value
	logBody       *pcommon.Value
	config        *Config
}

// TestRedactUnknownAttributes validates that the processor deletes span
// attributes that are not the allowed keys list
func TestRedactUnknownAttributes(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowedKeys: []string{"group", "id", "name"},
		},
		allowed: map[string]pcommon.Value{
			"group": pcommon.NewValueStr("temporary"),
			"id":    pcommon.NewValueInt(5),
			"name":  pcommon.NewValueStr("placeholder"),
		},
		ignored: map[string]pcommon.Value{
			"safe_attribute": pcommon.NewValueStr("4111111111111112"),
		},
		redacted: map[string]pcommon.Value{
			"credit_card": pcommon.NewValueStr("4111111111111111"),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)
	outMetricsGauge := runMetricsTest(t, tc, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, tc, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, tc, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, tc, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, tc, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).Resource().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).Resource().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map(),
		outMetricsGauge.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}
	for _, attr := range attrs {
		for k, v := range tc.allowed {
			val, ok := attr.Get(k)
			assert.True(t, ok)
			assert.Equal(t, v.AsRaw(), val.AsRaw())
		}
		for k := range tc.redacted {
			_, ok := attr.Get(k)
			assert.False(t, ok)
		}
	}
}

// TestAllowAllKeys validates that the processor does not delete
// span attributes that are not the allowed keys list if Config.AllowAllKeys
// is set to true
func TestAllowAllKeys(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowedKeys:  []string{"group", "id"},
			AllowAllKeys: true,
		},
		allowed: map[string]pcommon.Value{
			"group": pcommon.NewValueStr("temporary"),
			"id":    pcommon.NewValueInt(5),
			"name":  pcommon.NewValueStr("placeholder"),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)
	outMetricsGauge := runMetricsTest(t, tc, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, tc, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, tc, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, tc, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, tc, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).Resource().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).Resource().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map(),
		outMetricsGauge.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		for k, v := range tc.allowed {
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
	tc := testConfig{
		config: &Config{
			AllowedKeys:   []string{"group", "id", "name"},
			BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
			AllowAllKeys:  true,
		},
		allowed: map[string]pcommon.Value{
			"group": pcommon.NewValueStr("temporary"),
			"id":    pcommon.NewValueInt(5),
			"name":  pcommon.NewValueStr("placeholder"),
		},
		masked: map[string]pcommon.Value{
			"credit_card": pcommon.NewValueStr("placeholder 4111111111111111"),
		},
		allowedValues: map[string]pcommon.Value{
			"email": pcommon.NewValueStr("user@mycompany.com"),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)
	outMetricsGauge := runMetricsTest(t, tc, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, tc, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, tc, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, tc, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, tc, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).Resource().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).Resource().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map(),
		outMetricsGauge.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		for k, v := range tc.allowed {
			val, ok := attr.Get(k)
			assert.True(t, ok)
			assert.Equal(t, v.AsRaw(), val.AsRaw())
		}
		value, _ := attr.Get("credit_card")
		assert.Equal(t, "placeholder ****", value.Str())

		value, _ = attr.Get("email")
		assert.Equal(t, "user@mycompany.com", value.Str())
	}
}

// TODO: Test redaction with metric tags in a metrics PR

// TestRedactSummaryDebug validates that the processor writes a verbose summary
// of any attributes it deleted to the new redaction.redacted.keys and
// redaction.redacted.count span attributes while set to full debug output
func TestRedactSummaryDebug(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowedKeys:        []string{"id", "group", "name", "group.id", "member (id)", "token_some", "api_key_some", "email"},
			BlockedValues:      []string{"4[0-9]{12}(?:[0-9]{3})?"},
			IgnoredKeys:        []string{"safe_attribute"},
			IgnoredKeyPatterns: []string{"safeRE_attribute.*"},
			BlockedKeyPatterns: []string{".*(token|api_key).*"},
			AllowedValues:      []string{".+@mycompany.com"},
			Summary:            "debug",
		},
		allowed: map[string]pcommon.Value{
			"id":          pcommon.NewValueInt(5),
			"group.id":    pcommon.NewValueStr("some.valid.id"),
			"member (id)": pcommon.NewValueStr("some other valid id"),
		},
		masked: map[string]pcommon.Value{
			"name": pcommon.NewValueStr("placeholder 4111111111111111"),
		},
		ignored: map[string]pcommon.Value{
			"safe_attribute":          pcommon.NewValueStr("harmless 4111111111111112"),
			"safeRE_attribute_id":     pcommon.NewValueStr("safe id"),
			"safeRE_attribute_source": pcommon.NewValueStr("safe source"),
		},
		redacted: map[string]pcommon.Value{
			"credit_card": pcommon.NewValueStr("4111111111111111"),
		},
		blockedKeys: map[string]pcommon.Value{
			"token_some":   pcommon.NewValueStr("tokenize"),
			"api_key_some": pcommon.NewValueStr("apinize"),
		},
		allowedValues: map[string]pcommon.Value{
			"email": pcommon.NewValueStr("user@mycompany.com"),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)
	outMetricsGauge := runMetricsTest(t, tc, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, tc, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, tc, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, tc, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, tc, pmetric.MetricTypeSummary)
	outLogBody := getLogBodyWithDebugAttrs(outLogs)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).Resource().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).Resource().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogBody,
		outMetricsGauge.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		deleted := make([]string, 0, len(tc.redacted))
		for k := range tc.redacted {
			_, ok := attr.Get(k)
			assert.False(t, ok)
			deleted = append(deleted, k)
		}
		redactedKeys, ok := attr.Get(redactionRedactedKeys)
		assert.True(t, ok)
		sort.Strings(deleted)
		assert.Equal(t, strings.Join(deleted, ","), redactedKeys.Str())
		redactedKeyCount, ok := attr.Get(redactionRedactedCount)
		assert.True(t, ok)
		assert.Equal(t, int64(len(deleted)), redactedKeyCount.Int())

		ignoredKeyCount, ok := attr.Get(redactionIgnoredCount)
		assert.True(t, ok)
		assert.Equal(t, int64(len(tc.ignored)), ignoredKeyCount.Int())

		blockedKeys := []string{"api_key_some", "name", "token_some"}
		maskedKeys, ok := attr.Get(redactionMaskedKeys)
		assert.True(t, ok)
		assert.Equal(t, strings.Join(blockedKeys, ","), maskedKeys.Str())
		maskedKeyCount, ok := attr.Get(redactionMaskedCount)
		assert.True(t, ok)
		assert.Equal(t, int64(3), maskedKeyCount.Int())
		value, _ := attr.Get("name")
		assert.Equal(t, "placeholder ****", value.Str())

		allowedValueCount, ok := attr.Get(redactionAllowedCount)
		assert.True(t, ok)
		assert.Equal(t, int64(1), allowedValueCount.Int())
		value, _ = attr.Get("email")
		assert.Equal(t, "user@mycompany.com", value.Str())

		value, _ = attr.Get("api_key_some")
		assert.Equal(t, "****", value.Str())
		value, _ = attr.Get("token_some")
		assert.Equal(t, "****", value.Str())
	}
}

func getLogBodyWithDebugAttrs(outLogs plog.Logs) pcommon.Map {
	outLogBody := outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map()
	outLogAttr := outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()

	bodyRedactedKeys, ok := outLogAttr.Get(redactionBodyRedactedKeys)
	if ok {
		outLogBody.PutStr(redactionRedactedKeys, bodyRedactedKeys.Str())
	}
	bodyRedactedCount, ok := outLogAttr.Get(redactionBodyRedactedCount)
	if ok {
		outLogBody.PutInt(redactionRedactedCount, bodyRedactedCount.Int())
	}
	bodyMaskedKeys, ok := outLogAttr.Get(redactionBodyMaskedKeys)
	if ok {
		outLogBody.PutStr(redactionMaskedKeys, bodyMaskedKeys.Str())
	}
	bodyMaskedCount, ok := outLogAttr.Get(redactionBodyMaskedCount)
	if ok {
		outLogBody.PutInt(redactionMaskedCount, bodyMaskedCount.Int())
	}
	bodyAllowedKeys, ok := outLogAttr.Get(redactionBodyAllowedKeys)
	if ok {
		outLogBody.PutStr(redactionAllowedKeys, bodyAllowedKeys.Str())
	}
	bodyAllowedCount, ok := outLogAttr.Get(redactionBodyAllowedCount)
	if ok {
		outLogBody.PutInt(redactionAllowedCount, bodyAllowedCount.Int())
	}
	bodyIgnoredCount, ok := outLogAttr.Get(redactionBodyIgnoredCount)
	if ok {
		outLogBody.PutInt(redactionIgnoredCount, bodyIgnoredCount.Int())
	}
	return outLogBody
}

func TestRedactSummaryDebugHashMD5(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowedKeys:        []string{"id", "group", "name", "group.id", "member (id)", "token_some", "api_key_some", "email"},
			BlockedValues:      []string{"4[0-9]{12}(?:[0-9]{3})?"},
			HashFunction:       MD5,
			IgnoredKeys:        []string{"safe_attribute"},
			IgnoredKeyPatterns: []string{"safeRE_attribute.*"},
			BlockedKeyPatterns: []string{".*token.*", ".*api_key.*"},
			Summary:            "debug",
		},
		allowed: map[string]pcommon.Value{
			"id":          pcommon.NewValueInt(5),
			"group.id":    pcommon.NewValueStr("some.valid.id"),
			"member (id)": pcommon.NewValueStr("some other valid id"),
		},
		masked: map[string]pcommon.Value{
			"name": pcommon.NewValueStr("placeholder 4111111111111111"),
		},
		ignored: map[string]pcommon.Value{
			"safe_attribute":          pcommon.NewValueStr("harmless 4111111111111112"),
			"safeRE_attribute_id":     pcommon.NewValueStr("safe id"),
			"safeRE_attribute_source": pcommon.NewValueStr("safe source"),
		},
		redacted: map[string]pcommon.Value{
			"credit_card": pcommon.NewValueStr("4111111111111111"),
		},
		blockedKeys: map[string]pcommon.Value{
			"token_some":   pcommon.NewValueStr("tokenize"),
			"api_key_some": pcommon.NewValueStr("apinize"),
		},
		allowedValues: map[string]pcommon.Value{
			"email": pcommon.NewValueStr("user@mycompany.com"),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)
	outMetricsGauge := runMetricsTest(t, tc, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, tc, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, tc, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, tc, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, tc, pmetric.MetricTypeSummary)
	outLogBody := getLogBodyWithDebugAttrs(outLogs)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).Resource().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).Resource().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogBody,
		outMetricsGauge.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		deleted := make([]string, 0, len(tc.redacted))
		for k := range tc.redacted {
			_, ok := attr.Get(k)
			assert.False(t, ok)
			deleted = append(deleted, k)
		}
		redactedKeys, ok := attr.Get(redactionRedactedKeys)
		assert.True(t, ok)
		sort.Strings(deleted)
		assert.Equal(t, strings.Join(deleted, ","), redactedKeys.Str())
		maskedKeyCount, ok := attr.Get(redactionRedactedCount)
		assert.True(t, ok)
		assert.Equal(t, int64(len(deleted)), maskedKeyCount.Int())

		ignoredKeyCount, ok := attr.Get(redactionIgnoredCount)
		assert.True(t, ok)
		assert.Equal(t, int64(len(tc.ignored)), ignoredKeyCount.Int())

		blockedKeys := []string{"api_key_some", "name", "token_some"}
		maskedKeys, ok := attr.Get(redactionMaskedKeys)
		assert.True(t, ok)
		assert.Equal(t, strings.Join(blockedKeys, ","), maskedKeys.Str())
		maskedValueCount, ok := attr.Get(redactionMaskedCount)
		assert.True(t, ok)
		assert.Equal(t, int64(3), maskedValueCount.Int())
		value, _ := attr.Get("name")
		assert.Equal(t, "placeholder 5910f4ea0062a0e29afd3dccc741e3ce", value.Str())
		value, _ = attr.Get("api_key_some")
		assert.Equal(t, "93a699237950bde9eb9d25c7ead025f3", value.Str())
		value, _ = attr.Get("token_some")
		assert.Equal(t, "77e9ef3680c5518785ef0121d3884c3d", value.Str())
	}
}

// TestRedactSummaryInfo validates that the processor writes a verbose summary
// of any attributes it deleted to the new redaction.redacted.count span
// attribute (but not to redaction.redacted.keys) when set to the info level
// of output
func TestRedactSummaryInfo(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowedKeys:        []string{"id", "name", "group", "email"},
			BlockedValues:      []string{"4[0-9]{12}(?:[0-9]{3})?"},
			IgnoredKeys:        []string{"safe_attribute"},
			IgnoredKeyPatterns: []string{"safeRE_attribute.*"},
			AllowedValues:      []string{".+@mycompany.com"},
			Summary:            "info",
		},
		allowed: map[string]pcommon.Value{
			"id": pcommon.NewValueInt(5),
		},
		ignored: map[string]pcommon.Value{
			"safe_attribute":          pcommon.NewValueStr("harmless but suspicious 4111111111111141"),
			"safeRE_attribute_id":     pcommon.NewValueStr("safe id"),
			"safeRE_attribute_source": pcommon.NewValueStr("safe source"),
		},
		masked: map[string]pcommon.Value{
			"name": pcommon.NewValueStr("placeholder 4111111111111111"),
		},
		redacted: map[string]pcommon.Value{
			"credit_card": pcommon.NewValueStr("4111111111111111"),
		},
		allowedValues: map[string]pcommon.Value{
			"email": pcommon.NewValueStr("user@mycompany.com"),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)
	outMetricsGauge := runMetricsTest(t, tc, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, tc, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, tc, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, tc, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, tc, pmetric.MetricTypeSummary)
	outLogBody := getLogBodyWithDebugAttrs(outLogs)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).Resource().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).Resource().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogBody,
		outMetricsGauge.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		deleted := make([]string, 0, len(tc.redacted))
		for k := range tc.redacted {
			_, ok := attr.Get(k)
			assert.False(t, ok)
			deleted = append(deleted, k)
		}
		_, ok := attr.Get(redactionRedactedKeys)
		assert.False(t, ok)
		maskedKeyCount, ok := attr.Get(redactionRedactedCount)
		assert.True(t, ok)
		assert.Equal(t, int64(len(deleted)), maskedKeyCount.Int())
		_, ok = attr.Get(redactionMaskedKeys)
		assert.False(t, ok)

		maskedValueCount, ok := attr.Get(redactionMaskedCount)
		assert.True(t, ok)
		assert.Equal(t, int64(1), maskedValueCount.Int())
		value, _ := attr.Get("name")
		assert.Equal(t, "placeholder ****", value.Str())

		allowedValueCount, ok := attr.Get(redactionAllowedCount)
		assert.True(t, ok)
		assert.Equal(t, int64(1), allowedValueCount.Int())
		value, _ = attr.Get("email")
		assert.Equal(t, "user@mycompany.com", value.Str())

		ignoredKeyCount, ok := attr.Get(redactionIgnoredCount)
		assert.True(t, ok)
		assert.Equal(t, int64(3), ignoredKeyCount.Int())
		value, _ = attr.Get("safe_attribute")
		assert.Equal(t, "harmless but suspicious 4111111111111141", value.Str())
		value, _ = attr.Get("safeRE_attribute_id")
		assert.Equal(t, "safe id", value.Str())
		value, _ = attr.Get("safeRE_attribute_source")
		assert.Equal(t, "safe source", value.Str())
	}
}

// TestRedactSummarySilent validates that the processor does not create the
// summary attributes when set to silent
func TestRedactSummarySilent(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowedKeys:   []string{"id", "name", "group"},
			BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
			Summary:       "silent",
		},
		allowed: map[string]pcommon.Value{
			"id": pcommon.NewValueInt(5),
		},
		masked: map[string]pcommon.Value{
			"name": pcommon.NewValueStr("placeholder 4111111111111111"),
		},
		redacted: map[string]pcommon.Value{
			"credit_card": pcommon.NewValueStr("4111111111111111"),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)
	outMetricsGauge := runMetricsTest(t, tc, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, tc, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, tc, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, tc, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, tc, pmetric.MetricTypeSummary)
	outLogBody := getLogBodyWithDebugAttrs(outLogs)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).Resource().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).Resource().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogBody,
		outMetricsGauge.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		for k := range tc.redacted {
			_, ok := attr.Get(k)
			assert.False(t, ok)
		}
		_, ok := attr.Get(redactionRedactedKeys)
		assert.False(t, ok)
		_, ok = attr.Get(redactionRedactedCount)
		assert.False(t, ok)
		_, ok = attr.Get(redactionMaskedKeys)
		assert.False(t, ok)
		_, ok = attr.Get(redactionMaskedCount)
		assert.False(t, ok)
		value, _ := attr.Get("name")
		assert.Equal(t, "placeholder ****", value.Str())
	}
}

// TestRedactSummaryDefault validates that the processor does not create the
// summary attributes by default
func TestRedactSummaryDefault(t *testing.T) {
	tc := testConfig{
		config: &Config{AllowedKeys: []string{"id", "name", "group"}},
		allowed: map[string]pcommon.Value{
			"id": pcommon.NewValueInt(5),
		},
		ignored: map[string]pcommon.Value{
			"internal": pcommon.NewValueStr("a harmless 12 digits 4111111111111113"),
		},
		masked: map[string]pcommon.Value{
			"name": pcommon.NewValueStr("placeholder 4111111111111111"),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)
	outMetricsGauge := runMetricsTest(t, tc, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, tc, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, tc, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, tc, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, tc, pmetric.MetricTypeSummary)
	outLogBody := getLogBodyWithDebugAttrs(outLogs)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).Resource().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).Resource().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogBody,
		outMetricsGauge.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		_, ok := attr.Get(redactionRedactedKeys)
		assert.False(t, ok)
		_, ok = attr.Get(redactionRedactedCount)
		assert.False(t, ok)
		_, ok = attr.Get(redactionMaskedKeys)
		assert.False(t, ok)
		_, ok = attr.Get(redactionMaskedCount)
		assert.False(t, ok)
		_, ok = attr.Get(redactionIgnoredCount)
		assert.False(t, ok)
	}
}

// TestMultipleBlockValues validates that the processor can block multiple
// patterns
func TestMultipleBlockValues(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowedKeys:   []string{"id", "name", "mystery"},
			BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?", "(5[1-5][0-9]{3})"},
			Summary:       "debug",
		},
		allowed: map[string]pcommon.Value{
			"id":      pcommon.NewValueInt(5),
			"mystery": pcommon.NewValueStr("mystery 52000"),
		},
		masked: map[string]pcommon.Value{
			"name": pcommon.NewValueStr("placeholder 4111111111111111 52000"),
		},
		redacted: map[string]pcommon.Value{
			"credit_card": pcommon.NewValueStr("4111111111111111"),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)
	outMetricsGauge := runMetricsTest(t, tc, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, tc, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, tc, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, tc, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, tc, pmetric.MetricTypeSummary)
	outLogBody := getLogBodyWithDebugAttrs(outLogs)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).Resource().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).Resource().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogBody,
		outMetricsGauge.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		deleted := make([]string, 0, len(tc.redacted))
		for k := range tc.redacted {
			_, ok := attr.Get(k)
			assert.False(t, ok)
			deleted = append(deleted, k)
		}
		redactedKeys, ok := attr.Get(redactionRedactedKeys)
		assert.True(t, ok)
		assert.Equal(t, strings.Join(deleted, ","), redactedKeys.Str())
		redactedKeyCount, ok := attr.Get(redactionRedactedCount)
		assert.True(t, ok)
		assert.Equal(t, int64(len(deleted)), redactedKeyCount.Int())

		blockedKeys := []string{"name", "mystery"}
		maskedValues, ok := attr.Get(redactionMaskedKeys)
		assert.True(t, ok)
		sort.Strings(blockedKeys)
		assert.Equal(t, strings.Join(blockedKeys, ","), maskedValues.Str())
		assert.Equal(t, pcommon.ValueTypeStr, maskedValues.Type())
		assert.Equal(t, strings.Join(blockedKeys, ","), maskedValues.Str())
		maskedKeyCount, ok := attr.Get(redactionMaskedCount)
		assert.True(t, ok)
		assert.Equal(t, int64(len(blockedKeys)), maskedKeyCount.Int())
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
	processor, err := newRedaction(t.Context(), config, zaptest.NewLogger(t))
	require.NoError(t, err)

	attrs := pcommon.NewMap()
	assert.NoError(t, attrs.FromRaw(map[string]any{
		"id":                   5,
		"redundant":            1.2,
		"mystery":              "mystery ****",
		"credit_card":          "4111111111111111",
		redactionRedactedKeys:  "dropped_attr1,dropped_attr2",
		redactionRedactedCount: 2,
		redactionMaskedKeys:    "mystery",
		redactionMaskedCount:   1,
	}))
	processor.processAttrs(t.Context(), attrs)

	assert.Equal(t, 7, attrs.Len())
	val, found := attrs.Get(redactionRedactedKeys)
	assert.True(t, found)
	assert.Equal(t, "dropped_attr1,dropped_attr2,redundant", val.Str())
	val, found = attrs.Get(redactionRedactedCount)
	assert.True(t, found)
	assert.Equal(t, int64(3), val.Int())
	val, found = attrs.Get(redactionMaskedCount)
	assert.True(t, found)
	assert.Equal(t, int64(2), val.Int())
}

// TestRedactAllTypesFalse validates that not all types are redacted when the setting is false
func TestRedactAllTypesFalse(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowedKeys:    []string{"group", "id", "name"},
			AllowAllKeys:   true,
			BlockedValues:  []string{"4[0-9]{12}(?:[0-9]{3})?"},
			RedactAllTypes: false,
		},
		allowed: map[string]pcommon.Value{
			"group":           pcommon.NewValueStr("temporary"),
			"id":              pcommon.NewValueInt(5),
			"name":            pcommon.NewValueStr("placeholder"),
			"credit_card_int": pcommon.NewValueInt(4111111111111111),
		},
		masked: map[string]pcommon.Value{
			"credit_card": pcommon.NewValueStr("placeholder 4111111111111111"),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)
	outMetricsGauge := runMetricsTest(t, tc, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, tc, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, tc, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, tc, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, tc, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).Resource().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).Resource().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		for k, v := range tc.allowed {
			val, ok := attr.Get(k)
			assert.True(t, ok)
			assert.Equal(t, v.AsRaw(), val.AsRaw())
		}
		value, _ := attr.Get("credit_card")
		assert.Equal(t, "placeholder ****", value.Str())
	}
}

// TestRedactAllTypesTrue validates the redact all types setting ensures ints and maps can be redacted
func TestRedactAllTypesTrue(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowedKeys:    []string{"group", "id", "name"},
			AllowAllKeys:   true,
			BlockedValues:  []string{"4[0-9]{12}(?:[0-9]{3})?"},
			RedactAllTypes: true,
		},
		allowed: map[string]pcommon.Value{
			"group": pcommon.NewValueStr("temporary"),
			"id":    pcommon.NewValueInt(5),
			"name":  pcommon.NewValueStr("placeholder"),
		},
		masked: map[string]pcommon.Value{
			"credit_card":     pcommon.NewValueStr("placeholder 4111111111111111"),
			"credit_card_int": pcommon.NewValueInt(4111111111111111),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)
	outMetricsGauge := runMetricsTest(t, tc, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, tc, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, tc, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, tc, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, tc, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).Resource().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes(),
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).Resource().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).Resource().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		for k, v := range tc.allowed {
			val, ok := attr.Get(k)
			assert.True(t, ok)
			assert.Equal(t, v.AsRaw(), val.AsRaw())
		}
		value, _ := attr.Get("credit_card")
		assert.Equal(t, "placeholder ****", value.Str())

		value, _ = attr.Get("credit_card_int")
		assert.Equal(t, "****", value.Str())
	}
}

func TestSpanEventRedacted(t *testing.T) {
	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	library := ils.Scope()
	library.SetName("first-library")
	span := ils.Spans().AppendEmpty()
	span.SetName("first-batch-first-span")
	span.SetTraceID([16]byte{1, 2, 3, 4})

	event := span.Events().AppendEmpty()
	event.SetName("event-one")

	event.Attributes().PutStr("password", "xyzxyz")
	event.Attributes().PutStr("username", "foobar")

	config := &Config{
		AllowAllKeys:  true,
		BlockedValues: []string{"xyzxyz"},
		Summary:       "debug",
	}
	processor, err := newRedaction(t.Context(), config, zaptest.NewLogger(t))
	require.NoError(t, err)

	outTraces, err := processor.processTraces(t.Context(), inBatch)
	require.NoError(t, err)

	attr := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().At(0).Attributes()

	val, ok := attr.Get("password")
	require.True(t, ok)
	assert.Equal(t, "****", val.Str())

	val, ok = attr.Get("username")
	require.True(t, ok)
	require.Equal(t, "foobar", val.Str())
}

func TestLogBodyRedactionDifferentTypes(t *testing.T) {
	stringBody := pcommon.NewValueStr("placeholder 4111111111111111")
	tc := testConfig{
		config: &Config{
			AllowedKeys:   []string{"id", "email", "credit_card", "nested", "slice"},
			BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
			Summary:       "debug",
		},
		logBody: &stringBody,
	}

	outLogs := runLogsTest(t, tc)
	outLogBody := outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body()
	outLogAttrs := outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()
	assert.Equal(t, pcommon.ValueTypeStr, outLogBody.Type())
	assert.Equal(t, "placeholder ****", outLogBody.Str())
	val, found := outLogAttrs.Get(redactionBodyMaskedKeys)
	assert.True(t, found)
	assert.Equal(t, "body", val.Str())
	val, found = outLogAttrs.Get(redactionBodyMaskedCount)
	assert.True(t, found)
	assert.Equal(t, int64(1), val.Int())

	nestedBodyMap := pcommon.NewValueMap()
	nestedBodyMap.Map().PutStr("credit_card", "4111111111111111")
	nestedBodyMap.Map().PutStr("not_allowed_key", "temp")
	nestedBodyMap.Map().PutStr("id", "user123")

	innerMap := nestedBodyMap.Map().PutEmptyMap("nested")
	innerMap.PutStr("credit_card", "4111111111111111")
	innerMap.PutStr("not_allowed_key", "temp")
	innerMap.PutStr("id", "user123")
	innerMap.PutStr("safe_attribute", "4111111111111111")
	innerMap.PutStr("email", "user@mycompany.com")

	slice := nestedBodyMap.Map().PutEmptySlice("slice")
	slice.AppendEmpty().SetStr("4111111111111111")
	slice.AppendEmpty().SetStr("user123")

	tc = testConfig{
		config: &Config{
			AllowedKeys:   []string{"id", "email", "credit_card", "nested", "slice"},
			BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
			IgnoredKeys:   []string{"safe_attribute"},
			AllowedValues: []string{".+@mycompany.com"},
			Summary:       "debug",
		},
		logBody: &nestedBodyMap,
	}

	outLogs = runLogsTest(t, tc)

	outLogBody = outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body()
	assert.Equal(t, pcommon.ValueTypeMap, outLogBody.Type())

	creditCardValue, _ := outLogBody.Map().Get("credit_card")
	assert.Equal(t, "****", creditCardValue.Str())
	_, ok := outLogBody.Map().Get("not_allowed_key")
	assert.False(t, ok)
	id, ok := outLogBody.Map().Get("id")
	assert.True(t, ok)
	assert.Equal(t, "user123", id.Str())

	innerMapValue, _ := outLogBody.Map().Get("nested")
	assert.Equal(t, pcommon.ValueTypeMap, innerMapValue.Type())
	creditCardValue, _ = innerMapValue.Map().Get("credit_card")
	assert.Equal(t, "****", creditCardValue.Str())
	_, ok = innerMapValue.Map().Get("not_allowed_key")
	assert.False(t, ok)
	id, ok = innerMapValue.Map().Get("id")
	assert.True(t, ok)
	assert.Equal(t, "user123", id.Str())

	sliceValue, _ := outLogBody.Map().Get("slice")
	assert.Equal(t, pcommon.ValueTypeSlice, sliceValue.Type())
	assert.Equal(t, "****", sliceValue.Slice().At(0).Str())

	outLogAttrs = outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes()
	val, found = outLogAttrs.Get(redactionBodyRedactedKeys)
	assert.True(t, found)
	assert.Equal(t, "nested.not_allowed_key,not_allowed_key", val.Str())
	val, found = outLogAttrs.Get(redactionBodyRedactedCount)
	assert.True(t, found)
	assert.Equal(t, int64(2), val.Int())
	val, found = outLogAttrs.Get(redactionBodyMaskedKeys)
	assert.True(t, found)
	assert.Equal(t, "credit_card,nested.credit_card,slice.[0]", val.Str())
	val, found = outLogAttrs.Get(redactionBodyMaskedCount)
	assert.True(t, found)
	assert.Equal(t, int64(3), val.Int())
	val, found = outLogAttrs.Get(redactionBodyAllowedKeys)
	assert.True(t, found)
	assert.Equal(t, "nested.email", val.Str())
	val, found = outLogAttrs.Get(redactionBodyAllowedCount)
	assert.True(t, found)
	assert.Equal(t, int64(1), val.Int())
	val, found = outLogAttrs.Get(redactionBodyIgnoredCount)
	assert.True(t, found)
	assert.Equal(t, int64(1), val.Int())
}

// runTest transforms the test input data and passes it through the processor
func runTest(
	t *testing.T,
	cfg testConfig,
) ptrace.Traces {
	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	library := ils.Scope()
	library.SetName("first-library")
	span := ils.Spans().AppendEmpty()
	span.SetName("first-batch-first-span")
	span.SetTraceID([16]byte{1, 2, 3, 4})

	length := len(cfg.allowed) + len(cfg.masked) + len(cfg.redacted) + len(cfg.ignored) + len(cfg.blockedKeys) + len(cfg.allowedValues)
	for k, v := range cfg.allowed {
		v.CopyTo(span.Attributes().PutEmpty(k))
		v.CopyTo(rs.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.masked {
		v.CopyTo(span.Attributes().PutEmpty(k))
		v.CopyTo(rs.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.allowedValues {
		v.CopyTo(span.Attributes().PutEmpty(k))
		v.CopyTo(rs.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.redacted {
		v.CopyTo(span.Attributes().PutEmpty(k))
		v.CopyTo(rs.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.blockedKeys {
		v.CopyTo(span.Attributes().PutEmpty(k))
		v.CopyTo(rs.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.ignored {
		v.CopyTo(span.Attributes().PutEmpty(k))
		v.CopyTo(rs.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}

	assert.Equal(t, span.Attributes().Len(), length)
	assert.Equal(t, ils.Spans().At(0).Attributes().Len(), length)
	assert.Equal(t, inBatch.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Len(), length)

	// test
	ctx := t.Context()
	processor, err := newRedaction(ctx, cfg.config, zaptest.NewLogger(t))
	assert.NoError(t, err)
	outBatch, err := processor.processTraces(ctx, inBatch)

	// verify
	assert.NoError(t, err)
	return outBatch
}

// runLogsTest transforms the test input log data and passes it through the processor
func runLogsTest(
	t *testing.T,
	cfg testConfig,
) plog.Logs {
	inBatch := plog.NewLogs()
	rl := inBatch.ResourceLogs().AppendEmpty()
	ils := rl.ScopeLogs().AppendEmpty()
	_ = rl.ScopeLogs().AppendEmpty()

	library := ils.Scope()
	library.SetName("first-library")
	logEntry := ils.LogRecords().AppendEmpty()
	logEntry.SetTraceID([16]byte{1, 2, 3, 4})
	logEntry.Body().SetEmptyMap()

	length := len(cfg.allowed) + len(cfg.masked) + len(cfg.redacted) + len(cfg.ignored) + len(cfg.blockedKeys) + len(cfg.allowedValues)
	for k, v := range cfg.allowed {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
		v.CopyTo(logEntry.Body().Map().PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.masked {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
		v.CopyTo(logEntry.Body().Map().PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.allowedValues {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
		v.CopyTo(logEntry.Body().Map().PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.redacted {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
		v.CopyTo(logEntry.Body().Map().PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.blockedKeys {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
		v.CopyTo(logEntry.Body().Map().PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.ignored {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
		v.CopyTo(logEntry.Body().Map().PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	if cfg.logBody != nil {
		cfg.logBody.CopyTo(logEntry.Body())
	}

	assert.Equal(t, logEntry.Attributes().Len(), length)
	assert.Equal(t, ils.LogRecords().At(0).Attributes().Len(), length)
	assert.Equal(t, inBatch.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Len(), length)

	// test
	ctx := t.Context()
	processor, err := newRedaction(ctx, cfg.config, zaptest.NewLogger(t))
	assert.NoError(t, err)
	outBatch, err := processor.processLogs(ctx, inBatch)

	// verify
	assert.NoError(t, err)
	return outBatch
}

// runMetricsTest transforms the test input metric data and passes it through the processor
func runMetricsTest(
	t *testing.T,
	cfg testConfig,
	metricType pmetric.MetricType,
) pmetric.Metrics {
	inBatch := pmetric.NewMetrics()
	rl := inBatch.ResourceMetrics().AppendEmpty()
	ils := rl.ScopeMetrics().AppendEmpty()

	library := ils.Scope()
	library.SetName("first-library")
	metric := ils.Metrics().AppendEmpty()
	metric.SetDescription("first-batch-first-metric")

	length := len(cfg.allowed) + len(cfg.masked) + len(cfg.redacted) + len(cfg.ignored) + len(cfg.blockedKeys) + len(cfg.allowedValues)

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
	for k, v := range cfg.allowed {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.masked {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.allowedValues {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.redacted {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.blockedKeys {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.ignored {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
		v.CopyTo(ils.Scope().Attributes().PutEmpty(k))
	}

	assert.Equal(t, length, dataPointAttrs.Len())
	assert.Equal(t, length, rl.Resource().Attributes().Len())

	// test
	ctx := t.Context()
	processor, err := newRedaction(ctx, cfg.config, zaptest.NewLogger(t))
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
	ctx := b.Context()
	processor, _ := newRedaction(ctx, config, zaptest.NewLogger(b))

	for b.Loop() {
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
	ctx := b.Context()
	processor, _ := newRedaction(ctx, config, zaptest.NewLogger(b))

	for b.Loop() {
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

func TestURLSanitizationEnabled(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowAllKeys: true,
			URLSanitization: url.URLSanitizationConfig{
				Enabled:    true,
				Attributes: []string{"http.url", "url", "request_url"},
			},
			Summary: "debug",
		},
		allowed: map[string]pcommon.Value{
			"http.url":    pcommon.NewValueStr("/users/2"),
			"url":         pcommon.NewValueStr("/products/1/org/3"),
			"request_url": pcommon.NewValueStr("/v1/products/22"),
			"other_field": pcommon.NewValueStr("/not/sanitized/123"),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)
	outMetricsGauge := runMetricsTest(t, tc, pmetric.MetricTypeGauge)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		httpURL, _ := attr.Get("http.url")
		assert.Equal(t, "/users/*", httpURL.Str())

		url, _ := attr.Get("url")
		assert.Equal(t, "/products/*/org/*", url.Str())

		requestURL, _ := attr.Get("request_url")
		assert.Equal(t, "/v1/products/*", requestURL.Str())

		otherField, _ := attr.Get("other_field")
		assert.Equal(t, "/not/sanitized/123", otherField.Str())
	}
}

func TestURLSanitizationDisabled(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowAllKeys: true,
			URLSanitization: url.URLSanitizationConfig{
				Enabled: false,
			},
		},
		allowed: map[string]pcommon.Value{
			"http.url": pcommon.NewValueStr("/v1/products/123"),
			"url":      pcommon.NewValueStr("/users/456/profile"),
		},
	}

	outTraces := runTest(t, tc)
	outLogs := runLogsTest(t, tc)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
	}

	for _, attr := range attrs {
		httpURL, _ := attr.Get("http.url")
		assert.Equal(t, "/v1/products/123", httpURL.Str())

		url, _ := attr.Get("url")
		assert.Equal(t, "/users/456/profile", url.Str())
	}
}

func TestURLSanitizationWithBlockedValues(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowAllKeys:  true,
			BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
			URLSanitization: url.URLSanitizationConfig{
				Enabled:    true,
				Attributes: []string{"http.url"},
			},
			Summary: "debug",
		},
		allowed: map[string]pcommon.Value{
			"http.url":    pcommon.NewValueStr("/v1/products/2"),
			"credit_card": pcommon.NewValueStr("4111111111111111"),
		},
	}

	outTraces := runTest(t, tc)
	attr := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()

	httpURL, _ := attr.Get("http.url")
	assert.Equal(t, "/v1/products/*", httpURL.Str())

	creditCard, _ := attr.Get("credit_card")
	assert.Equal(t, "****", creditCard.Str())

	maskedCount, _ := attr.Get(redactionMaskedCount)
	assert.Equal(t, int64(2), maskedCount.Int())
}

func TestURLSanitizationInLogBody(t *testing.T) {
	bodyWithURL := pcommon.NewValueStr("/users/2")
	tc := testConfig{
		config: &Config{
			AllowAllKeys: true,
			URLSanitization: url.URLSanitizationConfig{
				Enabled: true,
			},
		},
		logBody: &bodyWithURL,
	}

	outLogs := runLogsTest(t, tc)
	outLogBody := outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body()

	assert.Equal(t, "/users/*", outLogBody.Str())
}

func TestURLSanitizationComplexLogBody(t *testing.T) {
	complexBody := pcommon.NewValueMap()
	complexBody.Map().PutStr("message", "/users/2")
	complexBody.Map().PutStr("url", "/products/1/org/3")

	tc := testConfig{
		config: &Config{
			AllowAllKeys: true,
			URLSanitization: url.URLSanitizationConfig{
				Enabled: true,
			},
		},
		logBody: &complexBody,
	}

	outLogs := runLogsTest(t, tc)
	outLogBody := outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body()

	message, _ := outLogBody.Map().Get("message")
	assert.Equal(t, "/users/*", message.Str())

	url, _ := outLogBody.Map().Get("url")
	assert.Equal(t, "/products/*/org/*", url.Str())
}

func TestURLSanitizationAttributeFiltering(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowAllKeys: true,
			URLSanitization: url.URLSanitizationConfig{
				Enabled:    true,
				Attributes: []string{"http.url"},
			},
		},
		allowed: map[string]pcommon.Value{
			"http.url":  pcommon.NewValueStr("/users/2"),
			"other_url": pcommon.NewValueStr("/users/3"),
		},
	}

	outTraces := runTest(t, tc)
	attr := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes()

	// Configured attribute should be sanitized
	httpURL, _ := attr.Get("http.url")
	assert.Equal(t, "/users/*", httpURL.Str())

	otherURL, _ := attr.Get("other_url")
	assert.Equal(t, "/users/3", otherURL.Str())
}

func TestDBObfuscationUsesDBSystemForAttributes(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowAllKeys: true,
			DBSanitizer: db.DBSanitizerConfig{
				SQLConfig: db.SQLConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
				RedisConfig: db.RedisConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
		},
	}

	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	sqlSpan := ils.Spans().AppendEmpty()
	sqlSpan.SetName("SELECT")
	sqlSpan.SetKind(ptrace.SpanKindClient)
	sqlSpan.Attributes().PutStr("db.system", "mysql")
	sqlSpan.Attributes().PutStr("db.statement", "SELECT id, email FROM users WHERE id = 42 AND email = 'foo@example.com'")

	redisSpan := ils.Spans().AppendEmpty()
	redisSpan.SetName("GET")
	redisSpan.SetKind(ptrace.SpanKindClient)
	redisSpan.Attributes().PutStr("db.system", "redis")
	redisSpan.Attributes().PutStr("db.statement", "SET user:12345 my-secret")

	processor, err := newRedaction(t.Context(), tc.config, zaptest.NewLogger(t))
	require.NoError(t, err)

	outTraces, err := processor.processTraces(t.Context(), inBatch)
	require.NoError(t, err)

	spans := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans()

	sqlStmt, ok := spans.At(0).Attributes().Get("db.statement")
	require.True(t, ok)
	assert.Equal(t, "SELECT id, email FROM users WHERE id = ? AND email = ?", sqlStmt.Str())

	redisStmt, ok := spans.At(1).Attributes().Get("db.statement")
	require.True(t, ok)
	assert.Equal(t, "SET user:12345 ?", redisStmt.Str())
}

func TestDBObfuscationUsesDBSystemNameForAttributes(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowAllKeys: true,
			DBSanitizer: db.DBSanitizerConfig{
				SQLConfig: db.SQLConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
		},
	}

	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	sqlSpan := ils.Spans().AppendEmpty()
	sqlSpan.SetName("SELECT")
	sqlSpan.SetKind(ptrace.SpanKindClient)
	sqlSpan.Attributes().PutStr("db.system.name", "postgresql")
	sqlSpan.Attributes().PutStr("db.statement", "SELECT email FROM users WHERE email = 'foo@bar.com'")

	processor, err := newRedaction(t.Context(), tc.config, zaptest.NewLogger(t))
	require.NoError(t, err)

	outTraces, err := processor.processTraces(t.Context(), inBatch)
	require.NoError(t, err)

	stmt, ok := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Get("db.statement")
	require.True(t, ok)
	assert.Equal(t, "SELECT email FROM users WHERE email = ?", stmt.Str())
}

func TestDBObfuscationAttributesWithoutDBSystemDoesNothing(t *testing.T) {
	tc := testConfig{
		config: &Config{
			AllowAllKeys: true,
			DBSanitizer: db.DBSanitizerConfig{
				SQLConfig: db.SQLConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
				RedisConfig: db.RedisConfig{
					Enabled:    true,
					Attributes: []string{"db.statement"},
				},
			},
		},
	}

	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()

	sqlSpan := ils.Spans().AppendEmpty()
	sqlSpan.SetName("SELECT")
	sqlSpan.SetKind(ptrace.SpanKindClient)
	sqlSpan.Attributes().PutStr("db.statement", "SELECT id FROM accounts WHERE id = 42")

	redisSpan := ils.Spans().AppendEmpty()
	redisSpan.SetName("GET")
	redisSpan.SetKind(ptrace.SpanKindClient)
	redisSpan.Attributes().PutStr("db.statement", "SET user:999 secret")

	processor, err := newRedaction(t.Context(), tc.config, zaptest.NewLogger(t))
	require.NoError(t, err)

	outTraces, err := processor.processTraces(t.Context(), inBatch)
	require.NoError(t, err)

	spans := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans()

	sqlStmt, ok := spans.At(0).Attributes().Get("db.statement")
	require.True(t, ok)
	assert.Equal(t, "SELECT id FROM accounts WHERE id = 42", sqlStmt.Str())

	redisStmt, ok := spans.At(1).Attributes().Get("db.statement")
	require.True(t, ok)
	assert.Equal(t, "SET user:999 secret", redisStmt.Str())
}

func TestLogAttributesObfuscationWithoutDBSystem(t *testing.T) {
	cfg := &Config{
		AllowAllKeys: true,
		DBSanitizer: db.DBSanitizerConfig{
			SQLConfig: db.SQLConfig{
				Enabled:    true,
				Attributes: []string{"db.statement"},
			},
		},
	}

	cfg.DBSanitizer.AllowFallbackWithoutSystem = true

	processor, err := newRedaction(t.Context(), cfg, zaptest.NewLogger(t))
	require.NoError(t, err)

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	ils := rl.ScopeLogs().AppendEmpty()
	logRecord := ils.LogRecords().AppendEmpty()
	logRecord.Attributes().PutStr("db.statement", "SELECT password FROM users WHERE id = 42")

	outLogs, err := processor.processLogs(t.Context(), logs)
	require.NoError(t, err)

	stmt, ok := outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("db.statement")
	require.True(t, ok)
	assert.Equal(t, "SELECT password FROM users WHERE id = ?", stmt.Str())
}

func TestMetricAttributesDBObfuscationWithSystem(t *testing.T) {
	cfg := &Config{
		AllowAllKeys: true,
		DBSanitizer: db.DBSanitizerConfig{
			SQLConfig: db.SQLConfig{
				Enabled:    true,
				Attributes: []string{"db.statement"},
			},
		},
	}

	processor, err := newRedaction(t.Context(), cfg, zaptest.NewLogger(t))
	require.NoError(t, err)

	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("request")
	metric.SetEmptyGauge()
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("db.system", "mysql")
	dp.Attributes().PutStr("db.statement", "SELECT id FROM accounts WHERE id = 42")

	outMetrics, err := processor.processMetrics(t.Context(), metrics)
	require.NoError(t, err)

	outAttrs := outMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes()
	stmt, ok := outAttrs.Get("db.statement")
	require.True(t, ok)
	assert.Equal(t, "SELECT id FROM accounts WHERE id = ?", stmt.Str())
}

func TestMetricAttributesDBObfuscationWithoutSystem(t *testing.T) {
	cfg := &Config{
		AllowAllKeys: true,
		DBSanitizer: db.DBSanitizerConfig{
			SQLConfig: db.SQLConfig{
				Enabled:    true,
				Attributes: []string{"db.statement"},
			},
		},
	}

	processor, err := newRedaction(t.Context(), cfg, zaptest.NewLogger(t))
	require.NoError(t, err)

	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("request")
	metric.SetEmptyGauge()
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("db.statement", "SELECT id FROM accounts WHERE id = 42")

	outMetrics, err := processor.processMetrics(t.Context(), metrics)
	require.NoError(t, err)

	outAttrs := outMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes()
	stmt, ok := outAttrs.Get("db.statement")
	require.True(t, ok)
	assert.Equal(t, "SELECT id FROM accounts WHERE id = 42", stmt.Str())
}

func TestSanitizeSpanNameFlag(t *testing.T) {
	t.Run("URL/default behavior", func(t *testing.T) {
		tc := testConfig{
			config: &Config{
				AllowAllKeys: true,
				URLSanitization: url.URLSanitizationConfig{
					Enabled: true,
				},
			},
		}

		inBatch := ptrace.NewTraces()
		rs := inBatch.ResourceSpans().AppendEmpty()
		ils := rs.ScopeSpans().AppendEmpty()
		span := ils.Spans().AppendEmpty()
		span.SetName("/users/123/profile")
		span.SetKind(ptrace.SpanKindClient)

		processor, err := newRedaction(t.Context(), tc.config, zaptest.NewLogger(t))
		require.NoError(t, err)
		outTraces, err := processor.processTraces(t.Context(), inBatch)
		require.NoError(t, err)

		outSpan := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
		assert.Equal(t, "/users/*/profile", outSpan.Name())
	})

	t.Run("URL/disabled", func(t *testing.T) {
		sanitizeSpanName := false
		tc := testConfig{
			config: &Config{
				AllowAllKeys: true,
				URLSanitization: url.URLSanitizationConfig{
					Enabled:          true,
					SanitizeSpanName: &sanitizeSpanName,
				},
			},
		}

		inBatch := ptrace.NewTraces()
		rs := inBatch.ResourceSpans().AppendEmpty()
		ils := rs.ScopeSpans().AppendEmpty()
		span := ils.Spans().AppendEmpty()
		span.SetName("/users/123/profile")
		span.SetKind(ptrace.SpanKindClient)

		processor, err := newRedaction(t.Context(), tc.config, zaptest.NewLogger(t))
		require.NoError(t, err)
		outTraces, err := processor.processTraces(t.Context(), inBatch)
		require.NoError(t, err)

		outSpan := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
		assert.Equal(t, "/users/123/profile", outSpan.Name())
	})

	t.Run("URL/explicitly enabled", func(t *testing.T) {
		sanitizeSpanName := true
		tc := testConfig{
			config: &Config{
				AllowAllKeys: true,
				URLSanitization: url.URLSanitizationConfig{
					Enabled:          true,
					SanitizeSpanName: &sanitizeSpanName,
				},
			},
		}

		inBatch := ptrace.NewTraces()
		rs := inBatch.ResourceSpans().AppendEmpty()
		ils := rs.ScopeSpans().AppendEmpty()
		span := ils.Spans().AppendEmpty()
		span.SetName("/users/123/profile")
		span.SetKind(ptrace.SpanKindClient)

		processor, err := newRedaction(t.Context(), tc.config, zaptest.NewLogger(t))
		require.NoError(t, err)
		outTraces, err := processor.processTraces(t.Context(), inBatch)
		require.NoError(t, err)

		outSpan := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
		assert.Equal(t, "/users/*/profile", outSpan.Name())
	})

	t.Run("DB/default behavior", func(t *testing.T) {
		tc := testConfig{
			config: &Config{
				AllowAllKeys: true,
				DBSanitizer: db.DBSanitizerConfig{
					SQLConfig: db.SQLConfig{
						Enabled: true,
					},
				},
			},
		}

		inBatch := ptrace.NewTraces()
		rs := inBatch.ResourceSpans().AppendEmpty()
		ils := rs.ScopeSpans().AppendEmpty()
		span := ils.Spans().AppendEmpty()
		span.SetName("SELECT * FROM users WHERE id = 123")
		span.SetKind(ptrace.SpanKindClient)
		span.Attributes().PutStr("db.system", "mysql")

		processor, err := newRedaction(t.Context(), tc.config, zaptest.NewLogger(t))
		require.NoError(t, err)
		outTraces, err := processor.processTraces(t.Context(), inBatch)
		require.NoError(t, err)

		outSpan := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
		assert.Equal(t, "SELECT * FROM users WHERE id = ?", outSpan.Name())
	})

	t.Run("DB/disabled", func(t *testing.T) {
		sanitizeSpanName := false
		tc := testConfig{
			config: &Config{
				AllowAllKeys: true,
				DBSanitizer: db.DBSanitizerConfig{
					SQLConfig: db.SQLConfig{
						Enabled: true,
					},
					SanitizeSpanName: &sanitizeSpanName,
				},
			},
		}

		inBatch := ptrace.NewTraces()
		rs := inBatch.ResourceSpans().AppendEmpty()
		ils := rs.ScopeSpans().AppendEmpty()
		span := ils.Spans().AppendEmpty()
		span.SetName("SELECT * FROM users WHERE id = 123")
		span.SetKind(ptrace.SpanKindClient)

		processor, err := newRedaction(t.Context(), tc.config, zaptest.NewLogger(t))
		require.NoError(t, err)
		outTraces, err := processor.processTraces(t.Context(), inBatch)
		require.NoError(t, err)

		outSpan := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
		assert.Equal(t, "SELECT * FROM users WHERE id = 123", outSpan.Name())
	})

	t.Run("DB/explicitly enabled", func(t *testing.T) {
		sanitizeSpanName := true
		tc := testConfig{
			config: &Config{
				AllowAllKeys: true,
				DBSanitizer: db.DBSanitizerConfig{
					SQLConfig: db.SQLConfig{
						Enabled: true,
					},
					SanitizeSpanName: &sanitizeSpanName,
				},
			},
		}

		inBatch := ptrace.NewTraces()
		rs := inBatch.ResourceSpans().AppendEmpty()
		ils := rs.ScopeSpans().AppendEmpty()
		span := ils.Spans().AppendEmpty()
		span.SetName("SELECT * FROM users WHERE id = 123")
		span.SetKind(ptrace.SpanKindClient)
		span.Attributes().PutStr("db.system", "mysql")

		processor, err := newRedaction(t.Context(), tc.config, zaptest.NewLogger(t))
		require.NoError(t, err)
		outTraces, err := processor.processTraces(t.Context(), inBatch)
		require.NoError(t, err)

		outSpan := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
		assert.Equal(t, "SELECT * FROM users WHERE id = ?", outSpan.Name())
	})

	t.Run("both URL and DB flags work independently", func(t *testing.T) {
		urlSanitizeSpanName := false
		dbSanitizeSpanName := false
		tc := testConfig{
			config: &Config{
				AllowAllKeys: true,
				URLSanitization: url.URLSanitizationConfig{
					Enabled:          true,
					SanitizeSpanName: &urlSanitizeSpanName,
				},
				DBSanitizer: db.DBSanitizerConfig{
					SQLConfig: db.SQLConfig{
						Enabled: true,
					},
					SanitizeSpanName: &dbSanitizeSpanName,
				},
			},
		}

		inBatch := ptrace.NewTraces()
		rs := inBatch.ResourceSpans().AppendEmpty()
		ils := rs.ScopeSpans().AppendEmpty()
		span := ils.Spans().AppendEmpty()
		span.SetName("/api/users/123")
		span.SetKind(ptrace.SpanKindClient)

		processor, err := newRedaction(t.Context(), tc.config, zaptest.NewLogger(t))
		require.NoError(t, err)
		outTraces, err := processor.processTraces(t.Context(), inBatch)
		require.NoError(t, err)

		outSpan := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
		assert.Equal(t, "/api/users/123", outSpan.Name())
	})
}

func TestURLSanitizationOnAttributes(t *testing.T) {
	cfg := &Config{
		AllowAllKeys: true,
		URLSanitization: url.URLSanitizationConfig{
			Enabled:    true,
			Attributes: []string{"http.url"},
		},
	}

	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetName("test-span")
	span.Attributes().PutStr("http.url", "/api/users/12345/profile")

	processor, err := newRedaction(t.Context(), cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	outTraces, err := processor.processTraces(t.Context(), inBatch)
	require.NoError(t, err)

	outSpan := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	val, ok := outSpan.Attributes().Get("http.url")
	require.True(t, ok)
	assert.Equal(t, "/api/users/*/profile", val.Str())
}

func TestDBObfuscationOnLogBody(t *testing.T) {
	cfg := &Config{
		AllowAllKeys: true,
		DBSanitizer: db.DBSanitizerConfig{
			SQLConfig: db.SQLConfig{
				Enabled: true,
			},
			AllowFallbackWithoutSystem: true,
		},
	}

	inLogs := plog.NewLogs()
	rl := inLogs.ResourceLogs().AppendEmpty()
	ils := rl.ScopeLogs().AppendEmpty()
	logRecord := ils.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("SELECT password FROM users WHERE id = 42")

	processor, err := newRedaction(t.Context(), cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	outLogs, err := processor.processLogs(t.Context(), inLogs)
	require.NoError(t, err)

	outLog := outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Equal(t, "SELECT password FROM users WHERE id = ?", outLog.Body().Str())
}

func TestURLSanitizationOnLogBody(t *testing.T) {
	cfg := &Config{
		AllowAllKeys: true,
		URLSanitization: url.URLSanitizationConfig{
			Enabled: true,
		},
	}

	inLogs := plog.NewLogs()
	rl := inLogs.ResourceLogs().AppendEmpty()
	ils := rl.ScopeLogs().AppendEmpty()
	logRecord := ils.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("/api/orders/12345/details")

	processor, err := newRedaction(t.Context(), cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	outLogs, err := processor.processLogs(t.Context(), inLogs)
	require.NoError(t, err)

	outLog := outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Equal(t, "/api/orders/*/details", outLog.Body().Str())
}

func TestDBObfuscationErrorInAttribute(t *testing.T) {
	cfg := &Config{
		AllowAllKeys: true,
		DBSanitizer: db.DBSanitizerConfig{
			SQLConfig: db.SQLConfig{
				Enabled:    true,
				Attributes: []string{"db.statement"},
			},
		},
	}

	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetName("test-span")
	span.Attributes().PutStr("db.system", "mysql")
	span.Attributes().PutStr("db.statement", "SELECT * FROM users WHERE id = 123")

	processor, err := newRedaction(t.Context(), cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	outTraces, err := processor.processTraces(t.Context(), inBatch)
	require.NoError(t, err)

	outSpan := outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	val, ok := outSpan.Attributes().Get("db.statement")
	require.True(t, ok)
	assert.Equal(t, "SELECT * FROM users WHERE id = ?", val.Str())
}
