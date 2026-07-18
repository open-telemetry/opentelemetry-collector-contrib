// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/internal/metadata"
)

func TestGenerateParentSpan(t *testing.T) {
	random := rand.Reader
	traceID := generateTraceID(random)
	spanInputs := &PICTSpanInputs{
		Parent:     SpanParentRoot,
		Tracestate: TraceStateEmpty,
		Kind:       SpanKindServer,
		Attributes: SpanAttrHTTPServer,
		Events:     SpanChildCountTwo,
		Links:      SpanChildCountOne,
		Status:     SpanStatusOk,
	}
	span := ptrace.NewSpan()
	fillSpan(traceID, pcommon.SpanID([8]byte{}), "/gotest-parent", spanInputs, random, span)
	assert.Equal(t, traceID, span.TraceID())
	assert.True(t, span.ParentSpanID().IsEmpty())
	assert.Equal(t, 11, span.Attributes().Len())
	assert.Equal(t, ptrace.StatusCodeOk, span.Status().Code())
}

func TestGenerateChildSpan(t *testing.T) {
	random := rand.Reader
	traceID := generateTraceID(random)
	parentID := generateSpanID(random)
	spanInputs := &PICTSpanInputs{
		Parent:     SpanParentChild,
		Tracestate: TraceStateEmpty,
		Kind:       SpanKindClient,
		Attributes: SpanAttrDatabaseSQL,
		Events:     SpanChildCountEmpty,
		Links:      SpanChildCountEmpty,
		Status:     SpanStatusOk,
	}
	span := ptrace.NewSpan()
	fillSpan(traceID, parentID, "get_test_info", spanInputs, random, span)
	assert.Equal(t, traceID, span.TraceID())
	assert.Equal(t, parentID, span.ParentSpanID())
	assert.Equal(t, 12, span.Attributes().Len())
	assert.Equal(t, ptrace.StatusCodeOk, span.Status().Code())
}

func TestGenerateSpans(t *testing.T) {
	random := rand.Reader
	count1 := 16
	spans := ptrace.NewSpanSlice()
	err := appendSpans(count1, "testdata/generated_pict_pairs_spans.txt", random, spans)
	assert.NoError(t, err)
	assert.Equal(t, count1, spans.Len())

	count2 := 256
	spans = ptrace.NewSpanSlice()
	err = appendSpans(count2, "testdata/generated_pict_pairs_spans.txt", random, spans)
	assert.NoError(t, err)
	assert.Equal(t, count2, spans.Len())

	count3 := 118
	spans = ptrace.NewSpanSlice()
	err = appendSpans(count3, "testdata/generated_pict_pairs_spans.txt", random, spans)
	assert.NoError(t, err)
	assert.Equal(t, count3, spans.Len())
}

func TestGenerateMessagingProducerSpanFeatureGates(t *testing.T) {
	testCases := []struct {
		name         string
		dontEmitV0   bool
		emitV1       bool
		expectedKeys []string
		absentKeys   []string
	}{
		{
			name:         "default_v0_only",
			dontEmitV0:   false,
			emitV1:       false,
			expectedKeys: []string{"messaging.destination"},
			absentKeys:   []string{"messaging.destination.name"},
		},
		{
			name:         "double_publish",
			dontEmitV0:   false,
			emitV1:       true,
			expectedKeys: []string{"messaging.destination", "messaging.destination.name"},
			absentKeys:   []string{},
		},
		{
			name:         "v1_only",
			dontEmitV0:   true,
			emitV1:       true,
			expectedKeys: []string{"messaging.destination.name"},
			absentKeys:   []string{"messaging.destination"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0MessagingConventionsFeatureGate.ID(), tc.dontEmitV0))
			require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1MessagingConventionsFeatureGate.ID(), tc.emitV1))

			t.Cleanup(func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0MessagingConventionsFeatureGate.ID(), false))
				require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1MessagingConventionsFeatureGate.ID(), false))
			})

			random := rand.Reader
			traceID := generateTraceID(random)
			spanInputs := &PICTSpanInputs{
				Parent:     SpanParentRoot,
				Tracestate: TraceStateEmpty,
				Kind:       SpanKindProducer,
				Attributes: SpanAttrMessagingProducer,
				Events:     SpanChildCountEmpty,
				Links:      SpanChildCountEmpty,
				Status:     SpanStatusOk,
			}
			span := ptrace.NewSpan()
			fillSpan(traceID, pcommon.SpanID([8]byte{}), "/gotest-parent", spanInputs, random, span)

			attrs := span.Attributes()
			for _, k := range tc.expectedKeys {
				_, ok := attrs.Get(k)
				assert.True(t, ok, "Expected attribute %s to be present", k)
			}
			for _, k := range tc.absentKeys {
				_, ok := attrs.Get(k)
				assert.False(t, ok, "Expected attribute %s to be absent", k)
			}
		})
	}
}
