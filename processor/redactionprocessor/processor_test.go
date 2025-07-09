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

type TestConfig struct {
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
	testConfig := TestConfig{
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

	outTraces := runTest(t, testConfig)
	outLogs := runLogsTest(t, testConfig)
	outMetricsGauge := runMetricsTest(t, testConfig, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, testConfig, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, testConfig, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}
	for _, attr := range attrs {
		for k, v := range testConfig.allowed {
			val, ok := attr.Get(k)
			assert.True(t, ok)
			assert.Equal(t, v.AsRaw(), val.AsRaw())
		}
		for k := range testConfig.redacted {
			_, ok := attr.Get(k)
			assert.False(t, ok)
		}
	}
}

// TestAllowAllKeys validates that the processor does not delete
// span attributes that are not the allowed keys list if Config.AllowAllKeys
// is set to true
func TestAllowAllKeys(t *testing.T) {
	testConfig := TestConfig{
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

	outTraces := runTest(t, testConfig)
	outLogs := runLogsTest(t, testConfig)
	outMetricsGauge := runMetricsTest(t, testConfig, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, testConfig, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, testConfig, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		for k, v := range testConfig.allowed {
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
	testConfig := TestConfig{
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

	outTraces := runTest(t, testConfig)
	outLogs := runLogsTest(t, testConfig)
	outMetricsGauge := runMetricsTest(t, testConfig, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, testConfig, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, testConfig, pmetric.MetricTypeSummary)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map(),
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		for k, v := range testConfig.allowed {
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
	testConfig := TestConfig{
		config: &Config{
			AllowedKeys:        []string{"id", "group", "name", "group.id", "member (id)", "token_some", "api_key_some", "email"},
			BlockedValues:      []string{"4[0-9]{12}(?:[0-9]{3})?"},
			IgnoredKeys:        []string{"safe_attribute"},
			BlockedKeyPatterns: []string{".*token.*", ".*api_key.*"},
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
			"safe_attribute": pcommon.NewValueStr("harmless 4111111111111112"),
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

	outTraces := runTest(t, testConfig)
	outLogs := runLogsTest(t, testConfig)
	outMetricsGauge := runMetricsTest(t, testConfig, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, testConfig, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, testConfig, pmetric.MetricTypeSummary)
	outLogBody := getLogBodyWithDebugAttrs(outLogs)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogBody,
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		deleted := make([]string, 0, len(testConfig.redacted))
		for k := range testConfig.redacted {
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
		assert.Equal(t, int64(len(testConfig.ignored)), ignoredKeyCount.Int())

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
	testConfig := TestConfig{
		config: &Config{
			AllowedKeys:        []string{"id", "group", "name", "group.id", "member (id)", "token_some", "api_key_some", "email"},
			BlockedValues:      []string{"4[0-9]{12}(?:[0-9]{3})?"},
			HashFunction:       MD5,
			IgnoredKeys:        []string{"safe_attribute"},
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
			"safe_attribute": pcommon.NewValueStr("harmless 4111111111111112"),
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

	outTraces := runTest(t, testConfig)
	outLogs := runLogsTest(t, testConfig)
	outMetricsGauge := runMetricsTest(t, testConfig, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, testConfig, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, testConfig, pmetric.MetricTypeSummary)
	outLogBody := getLogBodyWithDebugAttrs(outLogs)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogBody,
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		deleted := make([]string, 0, len(testConfig.redacted))
		for k := range testConfig.redacted {
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
		assert.Equal(t, int64(len(testConfig.ignored)), ignoredKeyCount.Int())

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
	testConfig := TestConfig{
		config: &Config{
			AllowedKeys:   []string{"id", "name", "group", "email"},
			BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
			IgnoredKeys:   []string{"safe_attribute"},
			AllowedValues: []string{".+@mycompany.com"},
			Summary:       "info",
		},
		allowed: map[string]pcommon.Value{
			"id": pcommon.NewValueInt(5),
		},
		ignored: map[string]pcommon.Value{
			"safe_attribute": pcommon.NewValueStr("harmless but suspicious 4111111111111141"),
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

	outTraces := runTest(t, testConfig)
	outLogs := runLogsTest(t, testConfig)
	outMetricsGauge := runMetricsTest(t, testConfig, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, testConfig, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, testConfig, pmetric.MetricTypeSummary)
	outLogBody := getLogBodyWithDebugAttrs(outLogs)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogBody,
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		deleted := make([]string, 0, len(testConfig.redacted))
		for k := range testConfig.redacted {
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
		assert.Equal(t, int64(1), ignoredKeyCount.Int())
		value, _ = attr.Get("safe_attribute")
		assert.Equal(t, "harmless but suspicious 4111111111111141", value.Str())
	}
}

// TestRedactSummarySilent validates that the processor does not create the
// summary attributes when set to silent
func TestRedactSummarySilent(t *testing.T) {
	testConfig := TestConfig{
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

	outTraces := runTest(t, testConfig)
	outLogs := runLogsTest(t, testConfig)
	outMetricsGauge := runMetricsTest(t, testConfig, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, testConfig, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, testConfig, pmetric.MetricTypeSummary)
	outLogBody := getLogBodyWithDebugAttrs(outLogs)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogBody,
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		for k := range testConfig.redacted {
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
	testConfig := TestConfig{
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

	outTraces := runTest(t, testConfig)
	outLogs := runLogsTest(t, testConfig)
	outMetricsGauge := runMetricsTest(t, testConfig, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, testConfig, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, testConfig, pmetric.MetricTypeSummary)
	outLogBody := getLogBodyWithDebugAttrs(outLogs)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogBody,
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
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
	testConfig := TestConfig{
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

	outTraces := runTest(t, testConfig)
	outLogs := runLogsTest(t, testConfig)
	outMetricsGauge := runMetricsTest(t, testConfig, pmetric.MetricTypeGauge)
	outMetricsSum := runMetricsTest(t, testConfig, pmetric.MetricTypeSum)
	outMetricsHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeHistogram)
	outMetricsExponentialHistogram := runMetricsTest(t, testConfig, pmetric.MetricTypeExponentialHistogram)
	outMetricsSummary := runMetricsTest(t, testConfig, pmetric.MetricTypeSummary)
	outLogBody := getLogBodyWithDebugAttrs(outLogs)

	attrs := []pcommon.Map{
		outTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes(),
		outLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes(),
		outLogBody,
		outMetricsGauge.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes(),
		outMetricsSum.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes(),
		outMetricsHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0).Attributes(),
		outMetricsExponentialHistogram.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0).Attributes(),
		outMetricsSummary.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Summary().DataPoints().At(0).Attributes(),
	}

	for _, attr := range attrs {
		deleted := make([]string, 0, len(testConfig.redacted))
		for k := range testConfig.redacted {
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
	processor, err := newRedaction(context.TODO(), config, zaptest.NewLogger(t))
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
	processor.processAttrs(context.TODO(), attrs)

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
	processor, err := newRedaction(context.TODO(), config, zaptest.NewLogger(t))
	require.NoError(t, err)

	outTraces, err := processor.processTraces(context.TODO(), inBatch)
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
	testConfig := TestConfig{
		config: &Config{
			AllowedKeys:   []string{"id", "email", "credit_card", "nested", "slice"},
			BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
			Summary:       "debug",
		},
		logBody: &stringBody,
	}

	outLogs := runLogsTest(t, testConfig)
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

	testConfig = TestConfig{
		config: &Config{
			AllowedKeys:   []string{"id", "email", "credit_card", "nested", "slice"},
			BlockedValues: []string{"4[0-9]{12}(?:[0-9]{3})?"},
			IgnoredKeys:   []string{"safe_attribute"},
			AllowedValues: []string{".+@mycompany.com"},
			Summary:       "debug",
		},
		logBody: &nestedBodyMap,
	}

	outLogs = runLogsTest(t, testConfig)

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
	cfg TestConfig,
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
	}
	for k, v := range cfg.masked {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}
	for k, v := range cfg.allowedValues {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}
	for k, v := range cfg.redacted {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}
	for k, v := range cfg.blockedKeys {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}
	for k, v := range cfg.redacted {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}
	for k, v := range cfg.ignored {
		v.CopyTo(span.Attributes().PutEmpty(k))
	}

	assert.Equal(t, span.Attributes().Len(), length)
	assert.Equal(t, ils.Spans().At(0).Attributes().Len(), length)
	assert.Equal(t, inBatch.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().Len(), length)

	// test
	ctx := context.Background()
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
	cfg TestConfig,
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
	}
	for k, v := range cfg.masked {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
		v.CopyTo(logEntry.Body().Map().PutEmpty(k))
	}
	for k, v := range cfg.allowedValues {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
		v.CopyTo(logEntry.Body().Map().PutEmpty(k))
	}
	for k, v := range cfg.redacted {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
		v.CopyTo(logEntry.Body().Map().PutEmpty(k))
	}
	for k, v := range cfg.blockedKeys {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
		v.CopyTo(logEntry.Body().Map().PutEmpty(k))
	}
	for k, v := range cfg.redacted {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
		v.CopyTo(logEntry.Body().Map().PutEmpty(k))
	}
	for k, v := range cfg.ignored {
		v.CopyTo(logEntry.Attributes().PutEmpty(k))
		v.CopyTo(logEntry.Body().Map().PutEmpty(k))
	}
	if cfg.logBody != nil {
		cfg.logBody.CopyTo(logEntry.Body())
	}

	assert.Equal(t, logEntry.Attributes().Len(), length)
	assert.Equal(t, ils.LogRecords().At(0).Attributes().Len(), length)
	assert.Equal(t, inBatch.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Len(), length)

	// test
	ctx := context.Background()
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
	cfg TestConfig,
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
	}
	for k, v := range cfg.masked {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.allowedValues {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.redacted {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.blockedKeys {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.redacted {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
	}
	for k, v := range cfg.ignored {
		v.CopyTo(dataPointAttrs.PutEmpty(k))
		v.CopyTo(rl.Resource().Attributes().PutEmpty(k))
	}

	assert.Equal(t, length, dataPointAttrs.Len())
	assert.Equal(t, length, rl.Resource().Attributes().Len())

	// test
	ctx := context.Background()
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
