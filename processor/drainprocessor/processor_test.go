// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/metadatatest"
)

func newTestProcessor(t *testing.T, cfg *Config) *drainProcessor {
	t.Helper()
	return newTestProcessorWithHost(t, cfg, componenttest.NewNopHost())
}

func newTestProcessorWithHost(t *testing.T, cfg *Config, host component.Host) *drainProcessor {
	t.Helper()
	set := processortest.NewNopSettings(metadata.Type)
	p, err := newDrainProcessor(set, cfg)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), host))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })
	return p
}

func makeLogRecord(body string) plog.Logs {
	ld := plog.NewLogs()
	lr := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Body().SetStr(body)
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return ld
}

func makeMapBodyLogRecord(msgField, msgValue string) plog.Logs {
	ld := plog.NewLogs()
	lr := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	m := lr.Body().SetEmptyMap()
	m.PutStr(msgField, msgValue)
	m.PutStr("level", "info")
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return ld
}

func getFirstRecord(ld plog.Logs) plog.LogRecord {
	return ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
}

// templateAttr returns the log.record.template attribute value for the first
// log record in ld, failing the test if it is absent.
func templateAttr(t *testing.T, ld plog.Logs) string {
	t.Helper()
	v, ok := getFirstRecord(ld).Attributes().Get("log.record.template")
	require.True(t, ok, "log.record.template attribute must be set")
	return v.Str()
}

// TestAnnotatesTemplate verifies that the template attribute is set after a
// single log record is processed.
func TestAnnotatesTemplate(t *testing.T) {
	p := newTestProcessor(t, createDefaultConfig().(*Config))

	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 10.0.0.1 on port 443"))
	require.NoError(t, err)

	assert.NotEmpty(t, templateAttr(t, out))
}

// TestSimilarLinesSameTemplate verifies that after enough similar lines have
// been processed, they all share the same abstracted template.
//
// The first 3 tokens must be identical for go-drain3's prefix tree to route
// all lines to the same leaf node.
func TestSimilarLinesSameTemplate(t *testing.T) {
	p := newTestProcessor(t, createDefaultConfig().(*Config))

	lines := []string{
		"connected to host 10.0.0.1 on port 443",
		"connected to host 192.168.1.1 on port 8080",
		"connected to host 172.16.0.1 on port 80",
	}

	var outs []plog.Logs
	for _, line := range lines {
		out, err := p.processLogs(t.Context(), makeLogRecord(line))
		require.NoError(t, err)
		outs = append(outs, out)
	}

	// The first line creates a new cluster with itself as the template; abstraction
	// kicks in once a second similar line is seen. Lines 1 and 2 should share the
	// same abstracted template.
	tmpl1 := templateAttr(t, outs[1])
	assert.Equal(t, tmpl1, templateAttr(t, outs[2]), "lines 1 and 2 should converge on the same template")
	assert.Contains(t, tmpl1, "<*>")
}

// TestCustomAttributeName verifies that the configured attribute key is used
// instead of the default.
func TestCustomAttributeName(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.TemplateAttribute = "my.template"
	p := newTestProcessor(t, cfg)

	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 10.0.0.1"))
	require.NoError(t, err)

	_, ok := getFirstRecord(out).Attributes().Get("my.template")
	assert.True(t, ok, "custom template_attribute key must be used")
}

// TestBodyFieldExtraction verifies that BodyField pulls the named field from a
// structured map body rather than using the full body string.
func TestBodyFieldExtraction(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.BodyField = "message"
	p := newTestProcessor(t, cfg)

	msgs := []string{
		"connected to host 10.0.0.1 on port 443",
		"connected to host 192.168.1.1 on port 8080",
		"connected to host 172.16.0.1 on port 80",
	}
	var lastOut plog.Logs
	for _, msg := range msgs {
		var err error
		lastOut, err = p.processLogs(t.Context(), makeMapBodyLogRecord("message", msg))
		require.NoError(t, err)
	}

	tmpl := templateAttr(t, lastOut)
	assert.NotContains(t, tmpl, "level", "template should be derived from the message field, not the full map")
	assert.Contains(t, tmpl, "<*>", "template should be abstracted after similar lines")
}

// TestEmptyBodySkipped verifies that empty log bodies do not receive template
// attributes.
func TestEmptyBodySkipped(t *testing.T) {
	p := newTestProcessor(t, createDefaultConfig().(*Config))

	out, err := p.processLogs(t.Context(), makeLogRecord(""))
	require.NoError(t, err)

	_, ok := getFirstRecord(out).Attributes().Get("log.record.template")
	assert.False(t, ok, "empty body should not produce template attribute")
}

// TestMultipleResourceLogs verifies that records across multiple resource log
// groups are all annotated.
func TestMultipleResourceLogs(t *testing.T) {
	p := newTestProcessor(t, createDefaultConfig().(*Config))

	ld := plog.NewLogs()
	for range 3 {
		lr := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		lr.Body().SetStr("heartbeat ping from server")
	}

	out, err := p.processLogs(t.Context(), ld)
	require.NoError(t, err)

	for i := 0; i < out.ResourceLogs().Len(); i++ {
		lr := out.ResourceLogs().At(i).ScopeLogs().At(0).LogRecords().At(0)
		_, ok := lr.Attributes().Get("log.record.template")
		assert.True(t, ok, "resource log group %d: record should have template attribute", i)
	}
}

// TestSeedTemplatesPrePopulateTree verifies that seed_templates establishes
// clusters before any live logs arrive, so the first matching live record gets
// a stable cluster ID.
func TestSeedTemplatesPrePopulateTree(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.SeedTemplates = []string{
		"connected to host <*> on port <*>",
	}
	p := newTestProcessor(t, cfg)

	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 10.0.0.1 on port 443"))
	require.NoError(t, err)

	tmpl := templateAttr(t, out)
	assert.Contains(t, tmpl, "<*>", "seeded template should match the live record")
}

// TestSeedLogsPrePopulateTree verifies that seed_logs trains the tree before
// any live logs arrive.
func TestSeedLogsPrePopulateTree(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.SeedLogs = []string{
		"connected to host 10.0.0.1 on port 443",
		"connected to host 192.168.1.1 on port 8080",
		"connected to host 172.16.0.1 on port 80",
	}
	p := newTestProcessor(t, cfg)

	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 10.10.10.10 on port 9000"))
	require.NoError(t, err)

	tmpl := templateAttr(t, out)
	assert.Contains(t, tmpl, "<*>", "template should already be abstracted from seed logs")
}

// TestEmptySeedEntriesSkipped verifies that blank entries in seed lists do not
// cause errors or get added to the tree.
func TestEmptySeedEntriesSkipped(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.SeedTemplates = []string{"", "   ", "connected to host <*> on port <*>"}
	cfg.SeedLogs = []string{"", "   "}

	p := newTestProcessor(t, cfg)

	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 10.0.0.1 on port 443"))
	require.NoError(t, err)
	assert.NotEmpty(t, templateAttr(t, out))
}

// TestWarmupMinClustersSuppress verifies that when warmup_min_clusters is set,
// records pass through immediately but are not annotated until the cluster
// threshold is reached.
func TestWarmupMinClustersSuppress(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.WarmupMinClusters = 2
	p := newTestProcessor(t, cfg)

	// First record: one cluster, below threshold — should pass through unannotated.
	out1, err := p.processLogs(t.Context(), makeLogRecord("connected to host 10.0.0.1 on port 443"))
	require.NoError(t, err)
	assert.Equal(t, 1, out1.LogRecordCount(), "record should pass through immediately, not buffered")
	_, ok := getFirstRecord(out1).Attributes().Get("log.record.template")
	assert.False(t, ok, "record should not be annotated during warmup")

	// Second record: distinct pattern, reaches threshold — should be annotated.
	out2, err := p.processLogs(t.Context(), makeLogRecord("disk write error on device sda"))
	require.NoError(t, err)
	assert.Equal(t, 1, out2.LogRecordCount(), "record should pass through immediately")
	_, ok = getFirstRecord(out2).Attributes().Get("log.record.template")
	assert.True(t, ok, "record should be annotated once threshold is reached")
}

// namedParam returns the string value at "<prefix>.<name>", failing the test
// if the attribute is missing.
func namedParam(t *testing.T, ld plog.Logs, prefix, name string) string {
	t.Helper()
	key := prefix + "." + name
	v, ok := getFirstRecord(ld).Attributes().Get(key)
	require.True(t, ok, "expected named parameter attribute %q", key)
	return v.Str()
}

// wildcardsAttr returns the wildcards slice attribute values, or nil if absent.
func wildcardsAttr(t *testing.T, ld plog.Logs) []string {
	t.Helper()
	v, ok := getFirstRecord(ld).Attributes().Get("log.record.template.wildcards")
	if !ok {
		return nil
	}
	require.Equal(t, pcommon.ValueTypeSlice, v.Type(), "wildcards attribute must be a slice")
	out := make([]string, 0, v.Slice().Len())
	for i := 0; i < v.Slice().Len(); i++ {
		out = append(out, v.Slice().At(i).Str())
	}
	return out
}

// ipRule is a mask rule reused across tests.
func ipRule() MaskingRule {
	return MaskingRule{Name: "ip", Pattern: `(\d{1,3}\.){3}\d{1,3}`}
}

// TestNoMasksNoWildcardsNoParams verifies that with the default config
// (no masking rules, wildcards off) no parameter or wildcards attributes
// are written even when a template abstracts.
func TestNoMasksNoWildcardsNoParams(t *testing.T) {
	p := newTestProcessor(t, createDefaultConfig().(*Config))
	for _, line := range []string{
		"connected to host 10.0.0.1 on port 443",
		"connected to host 192.168.1.1 on port 8080",
	} {
		_, err := p.processLogs(t.Context(), makeLogRecord(line))
		require.NoError(t, err)
	}
	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 172.16.0.1 on port 80"))
	require.NoError(t, err)
	attrs := getFirstRecord(out).Attributes()
	attrs.Range(func(k string, _ pcommon.Value) bool {
		assert.NotContains(t, k, "log.record.template.parameter", "no parameter attribute expected")
		return true
	})
	_, hasWild := attrs.Get("log.record.template.wildcards")
	assert.False(t, hasWild, "no wildcards attribute expected when EmitWildcards is false")
}

// TestMaskedTemplateEmitsDynamicParameters verifies that with a masking rule
// configured, the template surfaces the mask token and a dynamic attribute
// is written per matched mask name.
func TestMaskedTemplateEmitsDynamicParameters(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaskingRules = []MaskingRule{ipRule()}
	p := newTestProcessor(t, cfg)

	lines := []string{
		"connected to host 10.0.0.1 on port 443",
		"connected to host 192.168.1.1 on port 8080",
		"connected to host 172.16.0.1 on port 80",
	}
	var lastOut plog.Logs
	for _, line := range lines {
		var err error
		lastOut, err = p.processLogs(t.Context(), makeLogRecord(line))
		require.NoError(t, err)
	}

	tmpl := templateAttr(t, lastOut)
	assert.Contains(t, tmpl, "<ip>", "template should carry the mask token in place of matched values")
	assert.Contains(t, tmpl, "<*>", "port position should be a Drain wildcard")

	assert.Equal(t, "172.16.0.1", namedParam(t, lastOut, "log.record.template.parameter", "ip"))
	// Wildcards attribute is off by default, so the port position is not surfaced.
	assert.Nil(t, wildcardsAttr(t, lastOut))
}

// TestParametersLiteralTemplate verifies that with masking rules configured
// but a fully-literal template (no variable positions), no parameter
// attribute is written.
func TestParametersLiteralTemplate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaskingRules = []MaskingRule{ipRule()}
	p := newTestProcessor(t, cfg)

	// Single line with no IP: no mask match, no <*> abstraction.
	out, err := p.processLogs(t.Context(), makeLogRecord("disk write error on device sda"))
	require.NoError(t, err)
	tmpl := templateAttr(t, out)
	assert.NotContains(t, tmpl, "<*>")
	assert.NotContains(t, tmpl, "<ip>")
	_, hasIP := getFirstRecord(out).Attributes().Get("log.record.template.parameter.ip")
	assert.False(t, hasIP, "no mask token in template so no parameter attribute expected")
}

// TestCustomParameterKeyPrefix verifies that ParameterKeyPrefix controls the
// attribute key namespace for extracted parameters.
func TestCustomParameterKeyPrefix(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaskingRules = []MaskingRule{ipRule()}
	cfg.ParameterKeyPrefix = "my.p"
	p := newTestProcessor(t, cfg)

	for _, line := range []string{
		"connected to host 10.0.0.1 on port 443",
		"connected to host 192.168.1.1 on port 8080",
	} {
		_, err := p.processLogs(t.Context(), makeLogRecord(line))
		require.NoError(t, err)
	}
	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 172.16.0.1 on port 80"))
	require.NoError(t, err)

	assert.Equal(t, "172.16.0.1", namedParam(t, out, "my.p", "ip"))
	_, defaultKey := getFirstRecord(out).Attributes().Get("log.record.template.parameter.ip")
	assert.False(t, defaultKey, "default key must not be used when custom prefix is configured")
}

// TestParametersSuppressedDuringWarmup verifies that neither named parameters
// nor wildcards are written while warmup is in effect.
func TestParametersSuppressedDuringWarmup(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaskingRules = []MaskingRule{ipRule()}
	cfg.EmitWildcards = true
	cfg.WarmupMinClusters = 5
	p := newTestProcessor(t, cfg)

	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 10.0.0.1 on port 443"))
	require.NoError(t, err)
	_, hasTmpl := getFirstRecord(out).Attributes().Get("log.record.template")
	require.False(t, hasTmpl, "template suppressed during warmup")
	_, hasIP := getFirstRecord(out).Attributes().Get("log.record.template.parameter.ip")
	assert.False(t, hasIP, "named parameter must not be written during warmup")
	assert.Nil(t, wildcardsAttr(t, out), "wildcards must not be written during warmup")
}

// TestParametersWithExtraDelimiters verifies that body tokenisation honors
// ExtraDelimiters when extracting parameter values.
func TestParametersWithExtraDelimiters(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaskingRules = []MaskingRule{ipRule()}
	cfg.EmitWildcards = true
	cfg.ExtraDelimiters = []string{":"}
	p := newTestProcessor(t, cfg)

	// With ":" as a delimiter, "host:NAME" tokenises as ["host", "NAME"], so
	// the host name lands in its own token position and abstracts to <*>.
	for _, line := range []string{
		"connected to host:alpha from 10.0.0.1",
		"connected to host:beta from 192.168.1.1",
		"connected to host:gamma from 172.16.0.1",
	} {
		_, err := p.processLogs(t.Context(), makeLogRecord(line))
		require.NoError(t, err)
	}
	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host:delta from 8.8.8.8"))
	require.NoError(t, err)

	tmpl := templateAttr(t, out)
	require.Contains(t, tmpl, "<*>")
	require.Contains(t, tmpl, "<ip>")

	assert.Equal(t, "8.8.8.8", namedParam(t, out, "log.record.template.parameter", "ip"))
	assert.Equal(t, []string{"delta"}, wildcardsAttr(t, out))
}

// TestParametersFromBodyField verifies extraction works when BodyField pulls
// the message out of a structured map body.
func TestParametersFromBodyField(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaskingRules = []MaskingRule{ipRule()}
	cfg.BodyField = "message"
	p := newTestProcessor(t, cfg)

	for _, msg := range []string{
		"connected to host 10.0.0.1 on port 443",
		"connected to host 192.168.1.1 on port 8080",
	} {
		_, err := p.processLogs(t.Context(), makeMapBodyLogRecord("message", msg))
		require.NoError(t, err)
	}
	out, err := p.processLogs(t.Context(), makeMapBodyLogRecord("message", "connected to host 172.16.0.1 on port 80"))
	require.NoError(t, err)

	assert.Equal(t, "172.16.0.1", namedParam(t, out, "log.record.template.parameter", "ip"))
}

// TestMaskingRulesApplyInOrder verifies that earlier rules run first and
// their replacements are visible to later rules.
func TestMaskingRulesApplyInOrder(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaskingRules = []MaskingRule{
		{Name: "ip", Pattern: `(\d{1,3}\.){3}\d{1,3}`},
		{Name: "num", Pattern: `\d+`},
	}
	p := newTestProcessor(t, cfg)

	for _, line := range []string{
		"req 1 from 10.0.0.1",
		"req 2 from 192.168.1.1",
	} {
		_, err := p.processLogs(t.Context(), makeLogRecord(line))
		require.NoError(t, err)
	}
	out, err := p.processLogs(t.Context(), makeLogRecord("req 3 from 8.8.8.8"))
	require.NoError(t, err)

	tmpl := templateAttr(t, out)
	assert.Contains(t, tmpl, "<ip>", "ip should be masked whole, not eaten by num rule")
	assert.Contains(t, tmpl, "<num>", "num rule should still fire on other integers")

	assert.Equal(t, "3", namedParam(t, out, "log.record.template.parameter", "num"))
	assert.Equal(t, "8.8.8.8", namedParam(t, out, "log.record.template.parameter", "ip"))
}

// TestParametersAlignmentMismatchSkips verifies that when a mask pattern
// spans whitespace, alignment breaks and no parameter or wildcards
// attributes are written. The template attribute is still emitted.
func TestParametersAlignmentMismatchSkips(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaskingRules = []MaskingRule{{Name: "id", Pattern: `id: \d+`}}
	cfg.EmitWildcards = true
	p := newTestProcessor(t, cfg)

	for _, line := range []string{
		"request id: 111 done",
		"request id: 222 done",
	} {
		_, err := p.processLogs(t.Context(), makeLogRecord(line))
		require.NoError(t, err)
	}
	out, err := p.processLogs(t.Context(), makeLogRecord("request id: 333 done"))
	require.NoError(t, err)

	tmpl := templateAttr(t, out)
	assert.Contains(t, tmpl, "<id>", "template should still surface the mask token")
	_, hasID := getFirstRecord(out).Attributes().Get("log.record.template.parameter.id")
	assert.False(t, hasID, "named parameter must not be written on token-count mismatch")
	assert.Nil(t, wildcardsAttr(t, out), "wildcards must not be written on token-count mismatch")
}

// TestSeedLogsGoThroughMasking verifies that SeedLogs are masked identically
// to live records.
func TestSeedLogsGoThroughMasking(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaskingRules = []MaskingRule{ipRule()}
	cfg.SeedLogs = []string{
		"connected to host 10.0.0.1 on port 443",
		"connected to host 192.168.1.1 on port 8080",
		"connected to host 172.16.0.1 on port 80",
	}
	p := newTestProcessor(t, cfg)

	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 8.8.8.8 on port 22"))
	require.NoError(t, err)

	tmpl := templateAttr(t, out)
	assert.Contains(t, tmpl, "<ip>", "seed logs should have been masked before training")
	assert.Contains(t, tmpl, "<*>", "port position should be abstracted")

	assert.Equal(t, "8.8.8.8", namedParam(t, out, "log.record.template.parameter", "ip"))
}

// TestEmitWildcardsWithoutMasks verifies the discovery workflow: with no
// masking rules but EmitWildcards on, the wildcards slice surfaces all
// variable positions in template order.
func TestEmitWildcardsWithoutMasks(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.EmitWildcards = true
	p := newTestProcessor(t, cfg)

	for _, line := range []string{
		"connected to host 10.0.0.1 on port 443",
		"connected to host 192.168.1.1 on port 8080",
	} {
		_, err := p.processLogs(t.Context(), makeLogRecord(line))
		require.NoError(t, err)
	}
	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 172.16.0.1 on port 80"))
	require.NoError(t, err)

	assert.Equal(t, []string{"172.16.0.1", "80"}, wildcardsAttr(t, out))
}

// TestCustomWildcardsAttribute verifies WildcardsAttribute overrides the
// default key.
func TestCustomWildcardsAttribute(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.EmitWildcards = true
	cfg.WildcardsAttribute = "my.wildcards"
	p := newTestProcessor(t, cfg)

	for _, line := range []string{
		"connected to host 10.0.0.1 on port 443",
		"connected to host 192.168.1.1 on port 8080",
	} {
		_, err := p.processLogs(t.Context(), makeLogRecord(line))
		require.NoError(t, err)
	}
	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 172.16.0.1 on port 80"))
	require.NoError(t, err)

	_, ok := getFirstRecord(out).Attributes().Get("my.wildcards")
	assert.True(t, ok, "custom wildcards_attribute key must be used")
	_, defaultKey := getFirstRecord(out).Attributes().Get("log.record.template.wildcards")
	assert.False(t, defaultKey, "default key must not be used when custom is configured")
}

// TestDuplicateMaskFirstMatchWins verifies that when a mask name matches
// more than one position, only the first-matched value is written and
// the masks-duplicates counter is incremented once per record for that
// mask, tagged with the mask name.
func TestDuplicateMaskFirstMatchWins(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting

	cfg := createDefaultConfig().(*Config)
	cfg.MaskingRules = []MaskingRule{ipRule()}

	set := metadatatest.NewSettings(tel)
	p, err := newDrainProcessor(set, cfg)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(context.Background())) }) //nolint:usetesting

	// Body has two IPs. Same prefix on every line so drain routes them all
	// to the same leaf and abstracts consistently.
	for _, line := range []string{
		"traffic from 10.0.0.1 to 10.0.0.2",
		"traffic from 192.168.1.1 to 192.168.1.2",
	} {
		_, err = p.processLogs(t.Context(), makeLogRecord(line))
		require.NoError(t, err)
	}
	out, err := p.processLogs(t.Context(), makeLogRecord("traffic from 8.8.8.8 to 9.9.9.9"))
	require.NoError(t, err)

	tmpl := templateAttr(t, out)
	require.Contains(t, tmpl, "<ip>")

	// Only the first-position value survives.
	assert.Equal(t, "8.8.8.8", namedParam(t, out, "log.record.template.parameter", "ip"))

	// Every record has two <ip> tokens after masking, so the duplicate
	// counter increments once per record. Expected count: 3, tagged mask="ip".
	metadatatest.AssertEqualProcessorDrainMasksDuplicates(t, tel,
		[]metricdata.DataPoint[int64]{{
			Attributes: attribute.NewSet(attribute.String("mask", "ip")),
			Value:      3,
		}},
		metricdatatest.IgnoreTimestamp())
}

// TestWarmupMinClustersZeroDisabled verifies that warmup_min_clusters=0
// annotates from the first record (default behavior).
func TestWarmupMinClustersZeroDisabled(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.WarmupMinClusters = 0
	p := newTestProcessor(t, cfg)

	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 10.0.0.1 on port 443"))
	require.NoError(t, err)
	assert.NotEmpty(t, templateAttr(t, out), "should annotate from first record when warmup disabled")
}
