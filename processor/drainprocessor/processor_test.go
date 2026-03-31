// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor/internal/metadata"
)

func newTestProcessor(t *testing.T, cfg *Config) *drainProcessor {
	t.Helper()
	set := processortest.NewNopSettings(metadata.Type)
	p, err := newDrainProcessor(set, cfg)
	require.NoError(t, err)
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

// --- buffer warmup tests ---

func bufferCfg() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.WarmupMode = warmupModeBuffer
	cfg.WarmupMinClusters = 2
	cfg.WarmupBufferMaxLogs = 100
	return cfg
}

// TestBufferWarmupHoldsRecords verifies that records are not forwarded while
// cluster count is below WarmupMinClusters.
func TestBufferWarmupHoldsRecords(t *testing.T) {
	p := newTestProcessor(t, bufferCfg())

	out, err := p.processLogs(t.Context(), makeLogRecord("connected to host 10.0.0.1 on port 443"))
	require.NoError(t, err)
	assert.Equal(t, 0, out.LogRecordCount(), "records should be held in buffer during warmup")
}

// TestBufferWarmupFlushOnMinClusters verifies that once WarmupMinClusters
// distinct templates have been observed the buffer is flushed with all records
// annotated.
func TestBufferWarmupFlushOnMinClusters(t *testing.T) {
	p := newTestProcessor(t, bufferCfg())

	batches := []string{
		"connected to host 10.0.0.1 on port 443", // cluster 1
		"disk write error on device sda",         // cluster 2 — triggers flush
	}

	var allOut []plog.Logs
	for _, line := range batches {
		out, err := p.processLogs(t.Context(), makeLogRecord(line))
		require.NoError(t, err)
		allOut = append(allOut, out)
	}

	assert.Equal(t, 0, allOut[0].LogRecordCount(), "first batch should be held")
	assert.Equal(t, 2, allOut[1].LogRecordCount(), "flushed output should contain all buffered records")

	for i := 0; i < allOut[1].ResourceLogs().Len(); i++ {
		lr := allOut[1].ResourceLogs().At(i).ScopeLogs().At(0).LogRecords().At(0)
		_, ok := lr.Attributes().Get("log.record.template")
		assert.True(t, ok, "record %d should have template attribute after flush", i)
	}
}

// TestBufferWarmupFlushOnCap verifies that the buffer is flushed when
// WarmupBufferMaxLogs is reached even if WarmupMinClusters has not been met.
func TestBufferWarmupFlushOnCap(t *testing.T) {
	cfg := bufferCfg()
	cfg.WarmupMinClusters = 1000 // unreachably high
	cfg.WarmupBufferMaxLogs = 2
	p := newTestProcessor(t, cfg)

	out1, err := p.processLogs(t.Context(), makeLogRecord("connected to host 10.0.0.1 on port 443"))
	require.NoError(t, err)
	assert.Equal(t, 0, out1.LogRecordCount(), "first record should be buffered")

	out2, err := p.processLogs(t.Context(), makeLogRecord("disk write error on device sda"))
	require.NoError(t, err)
	assert.Equal(t, 2, out2.LogRecordCount(), "buffer cap reached: both records should be flushed")
}

// TestBufferWarmupPostFlushPassthrough verifies that records processed after
// warmup ends are passed through immediately without buffering.
func TestBufferWarmupPostFlushPassthrough(t *testing.T) {
	p := newTestProcessor(t, bufferCfg())

	// Trigger warmup flush (two distinct clusters).
	_, err := p.processLogs(t.Context(), makeLogRecord("connected to host 10.0.0.1 on port 443"))
	require.NoError(t, err)
	_, err = p.processLogs(t.Context(), makeLogRecord("disk write error on device sda"))
	require.NoError(t, err)

	out, err := p.processLogs(t.Context(), makeLogRecord("user alice logged in"))
	require.NoError(t, err)
	assert.Equal(t, 1, out.LogRecordCount(), "post-warmup records should pass through immediately")
	_, ok := getFirstRecord(out).Attributes().Get("log.record.template")
	assert.True(t, ok, "post-warmup records should be annotated")
}

// TestBufferWarmupRecordOrder verifies that flushed records maintain their
// original arrival order.
func TestBufferWarmupRecordOrder(t *testing.T) {
	p := newTestProcessor(t, bufferCfg())

	lines := []string{
		"connected to host 10.0.0.1 on port 443",
		"disk write error on device sda",
	}

	var lastOut plog.Logs
	for _, line := range lines {
		out, err := p.processLogs(t.Context(), makeLogRecord(line))
		require.NoError(t, err)
		lastOut = out
	}

	require.Equal(t, 2, lastOut.LogRecordCount())

	r0 := lastOut.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	r1 := lastOut.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().At(0)
	assert.Equal(t, "connected to host 10.0.0.1 on port 443", r0.Body().Str())
	assert.Equal(t, "disk write error on device sda", r1.Body().Str())
}
