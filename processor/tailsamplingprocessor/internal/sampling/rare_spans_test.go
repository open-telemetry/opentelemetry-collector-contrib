// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.9.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/cms"
)

type mockTimeProvider struct {
	seconds []int64
	nReq    int
}

func (f *mockTimeProvider) getCurSecond() int64 {
	v := f.seconds[f.nReq%len(f.seconds)]
	f.nReq++
	return v
}

func newMockTimer(sec ...int64) *mockTimeProvider {
	return &mockTimeProvider{
		seconds: append([]int64{}, sec...),
		nReq:    0,
	}
}

func TestRareSpansSamplerSimple(t *testing.T) {
	serviceName := "test_svc"
	spanName := "test_span"
	key := serviceName + ":" + spanName
	spanCnt := int64(1)

	tmProvider := newMockTimer(0)
	traceMock := newMockTrace(serviceName, []string{spanName}, spanCnt)
	cmsStub := cms.NewCmsStubWithCounts(1, cms.CntMap{key: 1})

	sampler := NewRareSpansSamplerWithCms(
		1,
		1,
		1,
		tmProvider,
		cmsStub,
		componenttest.NewNopTelemetrySettings(),
	)

	des, err := sampler.Evaluate(context.Background(), pcommon.TraceID{}, traceMock)

	assert.NoError(t, err)
	assert.Equal(t, Sampled, des)
	assert.Equal(t, 1, cmsStub.InsertionsWithCnt)
	assert.Equal(t, 0, cmsStub.CountReq)
	assert.Equal(t, 0, cmsStub.InsertionsReq)
	assert.Equal(t, 0, cmsStub.ClearCnt)
	assert.Len(t, cmsStub.InsertionsWithCntKeys, 1)
	assert.Equal(t, key, cmsStub.InsertionsWithCntKeys[0])
}

func TestRareSpansSamplerSampleOneSpanInTrace(t *testing.T) {
	serviceName := "test_svc"
	spanName1 := "test_span1"
	spanName2 := "test_span2"
	key1 := serviceName + ":" + spanName1
	key2 := serviceName + ":" + spanName2
	spanCnt := int64(1)

	tmProvider := newMockTimer(0)
	traceMock := newMockTrace(serviceName, []string{spanName1, spanName2}, spanCnt)
	cmsStub := cms.NewCmsStubWithCounts(1, cms.CntMap{key1: 2, key2: 1})

	sampler := NewRareSpansSamplerWithCms(
		1,
		1,
		1,
		tmProvider,
		cmsStub,
		componenttest.NewNopTelemetrySettings(),
	)

	des, err := sampler.Evaluate(context.Background(), pcommon.TraceID{}, traceMock)

	assert.NoError(t, err)
	assert.Equal(t, Sampled, des)
	assert.Equal(t, 2, cmsStub.InsertionsWithCnt)
	assert.Equal(t, 0, cmsStub.CountReq)
	assert.Equal(t, 0, cmsStub.InsertionsReq)
	assert.Equal(t, 0, cmsStub.ClearCnt)
	assert.Len(t, cmsStub.InsertionsWithCntKeys, 2)
	assert.Equal(t, key1, cmsStub.InsertionsWithCntKeys[0])
	assert.Equal(t, key2, cmsStub.InsertionsWithCntKeys[1])
}

func TestRareSpansSamplerFreqLimit(t *testing.T) {
	serviceName := "test_svc"
	spanName := "test_span"
	key := serviceName + ":" + spanName
	spanCnt := int64(1)

	tmProvider := newMockTimer(0)
	traceMock := newMockTrace(serviceName, []string{spanName}, spanCnt)

	testCases := []struct {
		caseName       string
		cmsReturnValue uint32
		decision       Decision
		cmsFreqLimit   uint32
	}{
		{
			caseName:       "below_limit",
			cmsReturnValue: 1,
			decision:       Sampled,
			cmsFreqLimit:   2,
		},
		{
			caseName:       "equal_to_limit",
			cmsReturnValue: 2,
			decision:       Sampled,
			cmsFreqLimit:   2,
		},

		{
			caseName:       "above_limit",
			cmsReturnValue: 3,
			decision:       NotSampled,
			cmsFreqLimit:   2,
		},
	}

	for _, tCase := range testCases {
		t.Run(tCase.caseName, func(t *testing.T) {
			cmsStub := cms.NewCmsStubWithCounts(1, cms.CntMap{key: tCase.cmsReturnValue})
			sampler := NewRareSpansSamplerWithCms(
				tCase.cmsFreqLimit,
				int64(tCase.cmsFreqLimit+1),
				int64(tCase.cmsFreqLimit+1),
				tmProvider,
				cmsStub,
				componenttest.NewNopTelemetrySettings(),
			)

			des, err := sampler.Evaluate(context.Background(), pcommon.TraceID{}, traceMock)
			assert.NoError(t, err)
			assert.Equal(t, tCase.decision, des)
		})
	}
}

func TestRareSpansSamplerKeyLenLimit(t *testing.T) {
	serviceName := "test_svc"
	spanCnt := int64(1)
	tmProvider := newMockTimer(0)

	testCases := []struct {
		caseName     string
		decision     Decision
		cmsFreqLimit uint32
		evalErr      error
		spanNames    []string
		cmsCounts    int
	}{
		{
			caseName:     "below_limit",
			decision:     Sampled,
			cmsFreqLimit: 1,
			spanNames:    []string{string(make([]byte, spanUniqIDBufferSize-len(serviceName)-2))},
			cmsCounts:    1,
		},
		{
			caseName:     "equal_to_limit",
			decision:     Sampled,
			cmsFreqLimit: 1,
			spanNames:    []string{string(make([]byte, spanUniqIDBufferSize-len(serviceName)-1))},
			cmsCounts:    1,
		},
		{
			caseName:     "above_limit",
			decision:     NotSampled,
			cmsFreqLimit: 1,
			spanNames:    []string{string(make([]byte, spanUniqIDBufferSize-len(serviceName)))},
			cmsCounts:    0,
		},

		{
			caseName:     "one_span_above_limit",
			decision:     Sampled,
			cmsFreqLimit: 1,
			spanNames: []string{
				string(make([]byte, spanUniqIDBufferSize-len(serviceName))),
				string(make([]byte, spanUniqIDBufferSize-len(serviceName)-1)),
			},
			cmsCounts: 1,
		},
	}

	for _, tCase := range testCases {
		t.Run(tCase.caseName, func(t *testing.T) {
			cmsStub := cms.NewEmptyCmsStub(1)
			sampler := NewRareSpansSamplerWithCms(
				tCase.cmsFreqLimit,
				int64(tCase.cmsFreqLimit+1),
				int64(tCase.cmsFreqLimit+1),
				tmProvider,
				cmsStub,
				componenttest.NewNopTelemetrySettings(),
			)

			traceMock := newMockTrace(serviceName, tCase.spanNames, spanCnt)
			des, err := sampler.Evaluate(context.Background(), pcommon.TraceID{}, traceMock)
			assert.NoError(t, err)
			assert.Equal(t, tCase.decision, des)
			assert.Equal(t, tCase.cmsCounts, cmsStub.InsertionsWithCnt)
		})
	}
}

func TestRareSpansSamplerProcessedLimitSameSecond(t *testing.T) {
	serviceName := "test_svc"
	spanName := "test_span"

	tmProvider := newMockTimer(0)

	testCases := []struct {
		caseName        string
		decision        Decision
		processingLimit int64
		spansInTrace    int64
		cmsFreqLimit    uint32
		cmsProbes       int
	}{
		{
			caseName:        "below_limit",
			decision:        Sampled,
			processingLimit: 2,
			spansInTrace:    1,
			cmsFreqLimit:    1,
			cmsProbes:       1,
		},
		{
			caseName:        "equal_to_limit",
			decision:        Sampled,
			processingLimit: 2,
			spansInTrace:    2,
			cmsFreqLimit:    1,
			cmsProbes:       1,
		},
		{
			caseName:        "above_to_limit",
			decision:        NotSampled,
			processingLimit: 2,
			spansInTrace:    3,
			cmsFreqLimit:    1,
			cmsProbes:       0,
		},
	}

	for _, tCase := range testCases {
		t.Run(tCase.caseName, func(t *testing.T) {
			traceMock := newMockTrace(serviceName, []string{spanName}, tCase.spansInTrace)
			cmsStub := cms.NewEmptyCmsStub(1)
			sampler := NewRareSpansSamplerWithCms(
				tCase.cmsFreqLimit,
				tCase.processingLimit+1,
				tCase.processingLimit,
				tmProvider,
				cmsStub,
				componenttest.NewNopTelemetrySettings(),
			)

			des, err := sampler.Evaluate(context.Background(), pcommon.TraceID{}, traceMock)
			assert.NoError(t, err)
			assert.Equal(t, tCase.decision, des)
			assert.Equal(t, tCase.cmsProbes, cmsStub.InsertionsWithCnt)
		})
	}
}

func TestRareSpansSamplerSampledLimitSameSecond(t *testing.T) {
	serviceName := "test_svc"
	spanName := "test_span"

	tmProvider := newMockTimer(0)

	testCases := []struct {
		caseName     string
		decision     Decision
		sampledLimit int64
		spansInTrace int64
		cmsFreqLimit uint32
		cmsProbes    int
	}{
		{
			caseName:     "below_limit",
			decision:     Sampled,
			sampledLimit: 2,
			spansInTrace: 1,
			cmsFreqLimit: 1,
			cmsProbes:    1,
		},
		{
			caseName:     "equal_to_limit",
			decision:     Sampled,
			sampledLimit: 2,
			spansInTrace: 2,
			cmsFreqLimit: 1,
			cmsProbes:    1,
		},
		{
			caseName:     "above_to_limit",
			decision:     NotSampled,
			sampledLimit: 2,
			spansInTrace: 3,
			cmsFreqLimit: 1,
			cmsProbes:    0,
		},
	}

	for _, tCase := range testCases {
		t.Run(tCase.caseName, func(t *testing.T) {
			traceMock := newMockTrace(serviceName, []string{spanName}, tCase.spansInTrace)
			cmsStub := cms.NewEmptyCmsStub(1)
			sampler := NewRareSpansSamplerWithCms(
				tCase.cmsFreqLimit,
				tCase.sampledLimit,
				tCase.sampledLimit+1,
				tmProvider,
				cmsStub,
				componenttest.NewNopTelemetrySettings(),
			)

			des, err := sampler.Evaluate(context.Background(), pcommon.TraceID{}, traceMock)
			assert.NoError(t, err)
			assert.Equal(t, tCase.decision, des)
			assert.Equal(t, tCase.cmsProbes, cmsStub.InsertionsWithCnt)
		})
	}
}

func TestRareSpansSamplerSampledLimitDifferentSeconds(t *testing.T) {
	serviceName := "test_svc"
	spanName := "test_span"

	tmProvider := newMockTimer(0, 1)

	testCases := []struct {
		caseName     string
		decisions    []Decision
		sampledLimit int64
		spansInTrace []int64
		cmsFreqLimit uint32
		cmsProbes    int
	}{
		{
			caseName:     "below_limit",
			decisions:    []Decision{Sampled, Sampled},
			sampledLimit: 1,
			spansInTrace: []int64{1, 1},
			cmsFreqLimit: 1,
			cmsProbes:    2,
		},
		{
			caseName:     "above_limit_below_limit",
			decisions:    []Decision{NotSampled, Sampled},
			sampledLimit: 1,
			spansInTrace: []int64{3, 1},
			cmsFreqLimit: 1,
			cmsProbes:    1,
		},
	}

	for _, tCase := range testCases {
		t.Run(tCase.caseName, func(t *testing.T) {
			cmsStub := cms.NewEmptyCmsStub(1)
			sampler := NewRareSpansSamplerWithCms(
				tCase.cmsFreqLimit,
				tCase.sampledLimit,
				tCase.sampledLimit+1,
				tmProvider,
				cmsStub,
				componenttest.NewNopTelemetrySettings(),
			)

			for i := 0; i < len(tCase.spansInTrace); i++ {
				traceMock := newMockTrace(serviceName, []string{spanName}, tCase.spansInTrace[i])
				des, err := sampler.Evaluate(context.Background(), pcommon.TraceID{}, traceMock)
				assert.NoError(t, err)
				assert.Equal(t, tCase.decisions[i], des)
			}
			assert.Equal(t, tCase.cmsProbes, cmsStub.InsertionsWithCnt)
		})
	}
}

func TestRareSpansSamplerProcessedLimitDifferentSeconds(t *testing.T) {
	serviceName := "test_svc"
	spanName := "test_span"

	tmProvider := newMockTimer(0, 1)

	testCases := []struct {
		caseName       string
		decisions      []Decision
		processedLimit int64
		spansInTrace   []int64
		cmsFreqLimit   uint32
		cmsProbes      int
	}{
		{
			caseName:       "below_limit",
			decisions:      []Decision{Sampled, Sampled},
			processedLimit: 1,
			spansInTrace:   []int64{1, 1},
			cmsFreqLimit:   1,
			cmsProbes:      2,
		},
		{
			caseName:       "above_limit_below_limit",
			decisions:      []Decision{NotSampled, Sampled},
			processedLimit: 1,
			spansInTrace:   []int64{3, 1},
			cmsFreqLimit:   1,
			cmsProbes:      1,
		},
	}

	for _, tCase := range testCases {
		t.Run(tCase.caseName, func(t *testing.T) {
			cmsStub := cms.NewEmptyCmsStub(1)
			sampler := NewRareSpansSamplerWithCms(
				tCase.cmsFreqLimit,
				tCase.processedLimit+1,
				tCase.processedLimit,
				tmProvider,
				cmsStub,
				componenttest.NewNopTelemetrySettings(),
			)

			for i := 0; i < len(tCase.spansInTrace); i++ {
				traceMock := newMockTrace(serviceName, []string{spanName}, tCase.spansInTrace[i])
				des, err := sampler.Evaluate(context.Background(), pcommon.TraceID{}, traceMock)
				assert.NoError(t, err)
				assert.Equal(t, tCase.decisions[i], des)
			}
			assert.Equal(t, tCase.cmsProbes, cmsStub.InsertionsWithCnt)
		})
	}
}

func newMockTrace(svcName string, spansNames []string, spansCnt int64) *TraceData {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr(semconv.AttributeServiceName, svcName)
	ils := rs.ScopeSpans().AppendEmpty()

	for i, sp := range spansNames {
		span := ils.Spans().AppendEmpty()
		span.SetName(sp)
		span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		span.SetSpanID([8]byte{byte(i), 2, 3, 4, 5, 6, 7, 8})
	}

	traceSpanCount := &atomic.Int64{}
	traceSpanCount.Store(spansCnt)

	return &TraceData{
		ReceivedBatches: traces,
		SpanCount:       traceSpanCount,
	}
}
