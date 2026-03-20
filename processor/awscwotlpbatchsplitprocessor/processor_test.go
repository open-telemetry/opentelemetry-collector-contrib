// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscwotlpbatchsplitprocessor

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awscwotlpbatchsplitprocessor/internal/metadata"
)

func newTestProcessor(t *testing.T, maxSize int, sink *consumertest.LogsSink) *awsCWOTLPBatchLogProcessor {
	t.Helper()
	return &awsCWOTLPBatchLogProcessor{
		logger:             zap.NewNop(),
		nextConsumer:       sink,
		maxRequestByteSize: maxSize,
		baseLogBufferSize:  defaultBaseLogBufferSize,
	}
}

func newLogs(logCount int, body pcommon.Value) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("test-scope")
	sl.Scope().SetVersion("1.0.0")
	for i := 0; i < logCount; i++ {
		lr := sl.LogRecords().AppendEmpty()
		body.CopyTo(lr.Body())
	}
	return ld
}

func strBody(s string) pcommon.Value {
	v := pcommon.NewValueEmpty()
	v.SetStr(s)
	return v
}

// estimates size of a log with an empty body to
// capture the fixed cost from the buffer constant, resource attributes, and
// scope name/version so tests can subtract it and assert only on the body's contribution
func calculateBaseOverhead(p *awsCWOTLPBatchLogProcessor) int {
	empty := newLogs(1, pcommon.NewValueEmpty())
	rl := empty.ResourceLogs().At(0)
	sl := rl.ScopeLogs().At(0)
	return p.estimateLogSize(rl.Resource(), sl.Scope(), sl.LogRecords().At(0))
}

func TestFactory(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, "awscwotlpbatchsplit", f.Type().String())

	cfg := f.CreateDefaultConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, defaultMaxRequestByteSize, cfg.(*Config).MaxRequestByteSize)
	assert.NoError(t, cfg.(*Config).Validate())
}

func TestCreateLogsProcessor(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	sink := new(consumertest.LogsSink)
	p, err := f.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, consumer.Capabilities{MutatesData: true}, p.Capabilities())
	assert.NoError(t, p.Start(t.Context(), nil))
	assert.NoError(t, p.Shutdown(t.Context()))
}

func TestEstimateUTF8Size(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{name: "ascii", input: "hello", expected: 5},
		{name: "mixed ascii and non-ascii", input: "café", expected: 3 + 1*4},
		{name: "non-ascii only", input: "深入", expected: 2 * 4},
		{name: "empty", input: "", expected: 0},
		{name: "mixed cjk and ascii", input: "深入 Python", expected: 2*4 + len(" Python")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, estimateUTF8Size(tt.input))
		})
	}
}

func TestEstimateLogSize_Primitive(t *testing.T) {
	sink := new(consumertest.LogsSink)
	p := newTestProcessor(t, defaultMaxRequestByteSize, sink)

	baseOverhead := calculateBaseOverhead(p)

	tests := []struct {
		name         string
		body         pcommon.Value
		expectedData int
	}{
		{name: "string", body: strBody("test"), expectedData: 4},
		{name: "int", body: func() pcommon.Value { v := pcommon.NewValueEmpty(); v.SetInt(1); return v }(), expectedData: 1},
		{name: "double", body: func() pcommon.Value { v := pcommon.NewValueEmpty(); v.SetDouble(1.2); return v }(), expectedData: 3},
		{name: "bool true", body: func() pcommon.Value { v := pcommon.NewValueEmpty(); v.SetBool(true); return v }(), expectedData: 4},
		{name: "bool false", body: func() pcommon.Value { v := pcommon.NewValueEmpty(); v.SetBool(false); return v }(), expectedData: 5},
		{name: "empty", body: pcommon.NewValueEmpty(), expectedData: 0},
		{name: "non-ascii string", body: strBody("深入 Python"), expectedData: 2*4 + len(" Python")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ld := newLogs(1, tt.body)
			rl := ld.ResourceLogs().At(0)
			sl := rl.ScopeLogs().At(0)
			size := p.estimateLogSize(rl.Resource(), sl.Scope(), sl.LogRecords().At(0))
			assert.Equal(t, tt.expectedData, size-baseOverhead)
		})
	}
}

func TestEstimateLogSize_WithAttributes(t *testing.T) {
	sink := new(consumertest.LogsSink)
	p := newTestProcessor(t, defaultMaxRequestByteSize, sink)

	baseOverhead := calculateBaseOverhead(p)

	ld := newLogs(1, strBody("test_body"))
	rl := ld.ResourceLogs().At(0)
	sl := rl.ScopeLogs().At(0)
	sl.LogRecords().At(0).Attributes().PutStr("attr_key", "attr_value")
	size := p.estimateLogSize(rl.Resource(), sl.Scope(), sl.LogRecords().At(0))

	expectedSize := len("test_body") + len("attr_key") + len("attr_value")
	assert.Equal(t, expectedSize, size-baseOverhead)
}

func TestEstimateLogSize_NestedMap(t *testing.T) {
	sink := new(consumertest.LogsSink)
	p := newTestProcessor(t, defaultMaxRequestByteSize, sink)

	baseOverhead := calculateBaseOverhead(p)

	bodyStr := strings.Repeat("X", 400)
	body := pcommon.NewValueEmpty()
	innerMap := body.SetEmptyMap()
	innerInner := innerMap.PutEmptyMap("key")
	innerInner.PutStr("key", bodyStr)

	ld := newLogs(1, body)
	rl := ld.ResourceLogs().At(0)
	sl := rl.ScopeLogs().At(0)
	size := p.estimateLogSize(rl.Resource(), sl.Scope(), sl.LogRecords().At(0))

	expectedData := len("key") + len("key") + len(bodyStr)
	assert.Equal(t, expectedData, size-baseOverhead)
}

func TestEstimateLogSize_NestedSlice(t *testing.T) {
	sink := new(consumertest.LogsSink)
	p := newTestProcessor(t, defaultMaxRequestByteSize, sink)

	baseOverhead := calculateBaseOverhead(p)

	bodyStr := strings.Repeat("X", 400)
	body := pcommon.NewValueEmpty()
	outerSlice := body.SetEmptySlice()
	innerSlice := outerSlice.AppendEmpty().SetEmptySlice()
	innerSlice.AppendEmpty().SetStr(bodyStr)

	ld := newLogs(1, body)
	rl := ld.ResourceLogs().At(0)
	sl := rl.ScopeLogs().At(0)
	size := p.estimateLogSize(rl.Resource(), sl.Scope(), sl.LogRecords().At(0))

	assert.Equal(t, len(bodyStr), size-baseOverhead)
}

func TestEstimateLogSize_DepthExceeded(t *testing.T) {
	sink := new(consumertest.LogsSink)
	p := newTestProcessor(t, defaultMaxRequestByteSize, sink)

	baseOverhead := calculateBaseOverhead(p)

	calculated := strings.Repeat("X", 400)
	huge := strings.Repeat("X", defaultMaxRequestByteSize)

	body := pcommon.NewValueEmpty()
	m := body.SetEmptyMap()
	m.PutStr("calculated", calculated)
	current := m.PutEmptyMap("d0")
	for i := 1; i < defaultMaxDepth; i++ {
		current = current.PutEmptyMap("d" + strconv.Itoa(i))
	}
	current.PutStr("hidden", huge)

	ld := newLogs(1, body)
	rl := ld.ResourceLogs().At(0)
	sl := rl.ScopeLogs().At(0)
	size := p.estimateLogSize(rl.Resource(), sl.Scope(), sl.LogRecords().At(0))

	// "hidden" key and huge vale should not be traversed or calculated
	assert.Greater(t, size-baseOverhead, len("calculated")+len(calculated))
	assert.Less(t, size-baseOverhead, len("calculated")+len(calculated)+len(huge))
}

func TestEstimateLogSize_EarlyReturnWhenExceedsMax(t *testing.T) {
	maxSize := 10000
	sink := new(consumertest.LogsSink)
	p := newTestProcessor(t, maxSize, sink)

	body := pcommon.NewValueEmpty()
	m := body.SetEmptyMap()
	m.PutStr("bigKey", strings.Repeat("X", maxSize))
	inner := m.PutEmptyMap("nested")
	inner.PutStr("hugeKey", strings.Repeat("X", maxSize*100))

	ld := newLogs(1, body)
	rl := ld.ResourceLogs().At(0)
	sl := rl.ScopeLogs().At(0)
	size := p.estimateLogSize(rl.Resource(), sl.Scope(), sl.LogRecords().At(0))

	assert.GreaterOrEqual(t, size, maxSize)
	assert.Less(t, size, maxSize*2)
}

func TestConsumeLogs_SingleBatchUnderLimit(t *testing.T) {
	sink := new(consumertest.LogsSink)
	p := newTestProcessor(t, defaultMaxRequestByteSize, sink)

	ld := newLogs(10, strBody(strings.Repeat("x", 100)))
	err := p.ConsumeLogs(t.Context(), ld)
	require.NoError(t, err)

	assert.Len(t, sink.AllLogs(), 1)
	assert.Equal(t, 10, sink.AllLogs()[0].LogRecordCount())
}

func TestConsumeLogs_AllLogsOversized(t *testing.T) {
	maxSize := 5000
	sink := new(consumertest.LogsSink)
	p := newTestProcessor(t, maxSize, sink)

	ld := newLogs(5, strBody(strings.Repeat("X", maxSize+1)))

	err := p.ConsumeLogs(t.Context(), ld)
	require.NoError(t, err)

	assert.Len(t, sink.AllLogs(), 5)
	for _, batch := range sink.AllLogs() {
		assert.Equal(t, 1, batch.LogRecordCount())
	}
}

func TestConsumeLogs_MixedSizes(t *testing.T) {
	// Per-log overhead with resource {"host.name":"test"}, scope "test", no version:
	//   2000 (base) + 4 (scope name) + 9 (resource key) + 4 (resource value) = 2017
	// Size smallBody so exactly 10 small logs fit per batch.
	perLogOverhead := defaultBaseLogBufferSize + len("test") + len("host.name") + len("test")
	maxSize := defaultMaxRequestByteSize
	smallBodySize := maxSize/10 - perLogOverhead

	sink := new(consumertest.LogsSink)
	p := newTestProcessor(t, maxSize, sink)

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("host.name", "test")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("test")

	largeBody := strings.Repeat("X", maxSize+1)
	smallBody := strings.Repeat("x", smallBodySize)

	for i := 0; i < 3; i++ {
		sl.LogRecords().AppendEmpty().Body().SetStr(largeBody)
	}
	for i := 0; i < 12; i++ {
		sl.LogRecords().AppendEmpty().Body().SetStr(smallBody)
	}

	err := p.ConsumeLogs(t.Context(), ld)
	require.NoError(t, err)

	// 5 export calls should contain batch sizes [1, 1, 1, 10, 2] respectively
	require.Len(t, sink.AllLogs(), 5)
	expectedBatchSizes := []int{1, 1, 1, 10, 2}
	for i, expected := range expectedBatchSizes {
		assert.Equal(t, expected, sink.AllLogs()[i].LogRecordCount(), "batch %d", i)
	}
}

func TestConsumeLogs_EmptyLogs(t *testing.T) {
	sink := new(consumertest.LogsSink)
	p := newTestProcessor(t, defaultMaxRequestByteSize, sink)

	ld := plog.NewLogs()
	err := p.ConsumeLogs(t.Context(), ld)
	require.NoError(t, err)

	assert.Len(t, sink.AllLogs(), 1)
	assert.Equal(t, 0, sink.AllLogs()[0].LogRecordCount())
}

func TestConsumeLogs_PreservesResourceAndScope(t *testing.T) {
	sink := new(consumertest.LogsSink)
	p := newTestProcessor(t, 5000, sink)

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "my-service")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("my-scope")
	sl.Scope().SetVersion("1.2.3")

	for i := 0; i < 2; i++ {
		sl.LogRecords().AppendEmpty().Body().SetStr(strings.Repeat("X", 6000))
	}

	err := p.ConsumeLogs(t.Context(), ld)
	require.NoError(t, err)

	assert.Len(t, sink.AllLogs(), 2)
	for _, batch := range sink.AllLogs() {
		r := batch.ResourceLogs().At(0).Resource()
		v, ok := r.Attributes().Get("service.name")
		require.True(t, ok)
		assert.Equal(t, "my-service", v.Str())

		s := batch.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
		assert.Equal(t, "my-scope", s.Name())
		assert.Equal(t, "1.2.3", s.Version())
	}
}

func TestConsumeLogs_ConsumerError(t *testing.T) {
	errConsumer := consumertest.NewErr(errors.New("consumer error"))
	p := &awsCWOTLPBatchLogProcessor{
		logger:             zap.NewNop(),
		nextConsumer:       errConsumer,
		maxRequestByteSize: defaultMaxRequestByteSize,
		baseLogBufferSize:  defaultBaseLogBufferSize,
	}

	ld := newLogs(2, strBody(strings.Repeat("x", 100)))
	err := p.ConsumeLogs(t.Context(), ld)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "consumer error")
}
