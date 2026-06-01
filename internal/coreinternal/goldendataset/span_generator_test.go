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

func TestHTTPV1ProtocolVersionAttribute(t *testing.T) {
	prevDontEmit := metadata.InternalCoreinternalGoldendatasetDontEmitV0HTTPConventionsFeatureGate.IsEnabled()
	prevEmitV1 := metadata.InternalCoreinternalGoldendatasetEmitV1HTTPConventionsFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0HTTPConventionsFeatureGate.ID(), true))
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1HTTPConventionsFeatureGate.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0HTTPConventionsFeatureGate.ID(), prevDontEmit))
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1HTTPConventionsFeatureGate.ID(), prevEmitV1))
	})

	span := ptrace.NewSpan()
	appendHTTPServerAttributes(true, span.Attributes())

	_, hasLegacy := span.Attributes().Get("http.flavor")
	assert.False(t, hasLegacy)
	nameValue, hasName := span.Attributes().Get("network.protocol.name")
	if assert.True(t, hasName) {
		assert.Equal(t, "http", nameValue.AsString())
	}
	value, hasV1 := span.Attributes().Get("network.protocol.version")
	if assert.True(t, hasV1) {
		assert.Equal(t, "2", value.AsString())
	}
}

func TestMessagingV1DestinationNameAttribute(t *testing.T) {
	prevDontEmit := metadata.InternalCoreinternalGoldendatasetDontEmitV0MessagingConventionsFeatureGate.IsEnabled()
	prevEmitV1 := metadata.InternalCoreinternalGoldendatasetEmitV1MessagingConventionsFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0MessagingConventionsFeatureGate.ID(), true))
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1MessagingConventionsFeatureGate.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0MessagingConventionsFeatureGate.ID(), prevDontEmit))
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1MessagingConventionsFeatureGate.ID(), prevEmitV1))
	})

	span := ptrace.NewSpan()
	appendMessagingProducerAttributes(span.Attributes())

	_, hasLegacyDestination := span.Attributes().Get("messaging.destination")
	_, hasLegacyKind := span.Attributes().Get("messaging.destination.kind")
	assert.False(t, hasLegacyDestination)
	assert.False(t, hasLegacyKind)
	value, hasV1 := span.Attributes().Get("messaging.destination.name")
	if assert.True(t, hasV1) {
		assert.Equal(t, "time.us.east.atlanta", value.AsString())
	}
}
