// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/internal/metadata"
)

func TestGenerateTraces(t *testing.T) {
	rscSpans, err := GenerateTraces("testdata/generated_pict_pairs_traces.txt",
		"testdata/generated_pict_pairs_spans.txt")
	assert.NoError(t, err)
	assert.Len(t, rscSpans, 32)
}

// TestGenerateTracesIsReproducible verifies that the seeded random number generator produces the
// same trace, span, parent and link IDs on every call, which is what makes the golden dataset
// usable for reproducible tests.
func TestGenerateTracesIsReproducible(t *testing.T) {
	collectIDs := func(t *testing.T) []string {
		t.Helper()
		tds, err := GenerateTraces("testdata/generated_pict_pairs_traces.txt",
			"testdata/generated_pict_pairs_spans.txt")
		require.NoError(t, err)

		var ids []string
		for _, td := range tds {
			for i := 0; i < td.ResourceSpans().Len(); i++ {
				scopeSpans := td.ResourceSpans().At(i).ScopeSpans()
				for j := 0; j < scopeSpans.Len(); j++ {
					spans := scopeSpans.At(j).Spans()
					for k := 0; k < spans.Len(); k++ {
						span := spans.At(k)
						ids = append(ids, span.TraceID().String(), span.SpanID().String(), span.ParentSpanID().String())
						for l := 0; l < span.Links().Len(); l++ {
							link := span.Links().At(l)
							ids = append(ids, link.TraceID().String(), link.SpanID().String())
						}
					}
				}
			}
		}
		return ids
	}

	first := collectIDs(t)
	require.NotEmpty(t, first)
	assert.Equal(t, first, collectIDs(t))
}

func TestGenerateTracesInvalidRPCFeatureGateCombination(t *testing.T) {
	prevDontEmit := metadata.InternalCoreinternalGoldendatasetDontEmitV0RPCConventionsFeatureGate.IsEnabled()
	prevEmitV1 := metadata.InternalCoreinternalGoldendatasetEmitV1RPCConventionsFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0RPCConventionsFeatureGate.ID(), true))
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1RPCConventionsFeatureGate.ID(), false))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0RPCConventionsFeatureGate.ID(), prevDontEmit))
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1RPCConventionsFeatureGate.ID(), prevEmitV1))
	})

	_, err := GenerateTraces("testdata/generated_pict_pairs_traces.txt", "testdata/generated_pict_pairs_spans.txt")
	require.ErrorContains(t, err, "internal.coreinternal.goldendataset.DontEmitV0RPCConventions cannot be enabled without enabling internal.coreinternal.goldendataset.EmitV1RPCConventions")
}

func TestGenerateTracesInvalidHTTPFeatureGateCombination(t *testing.T) {
	prevDontEmit := metadata.InternalCoreinternalGoldendatasetDontEmitV0HTTPConventionsFeatureGate.IsEnabled()
	prevEmitV1 := metadata.InternalCoreinternalGoldendatasetEmitV1HTTPConventionsFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0HTTPConventionsFeatureGate.ID(), true))
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1HTTPConventionsFeatureGate.ID(), false))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0HTTPConventionsFeatureGate.ID(), prevDontEmit))
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1HTTPConventionsFeatureGate.ID(), prevEmitV1))
	})

	_, err := GenerateTraces("testdata/generated_pict_pairs_traces.txt", "testdata/generated_pict_pairs_spans.txt")
	require.ErrorContains(t, err, "internal.coreinternal.goldendataset.DontEmitV0HTTPConventions cannot be enabled without enabling internal.coreinternal.goldendataset.EmitV1HTTPConventions")
}

func TestGenerateTracesInvalidMessagingFeatureGateCombination(t *testing.T) {
	prevDontEmit := metadata.InternalCoreinternalGoldendatasetDontEmitV0MessagingConventionsFeatureGate.IsEnabled()
	prevEmitV1 := metadata.InternalCoreinternalGoldendatasetEmitV1MessagingConventionsFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0MessagingConventionsFeatureGate.ID(), true))
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1MessagingConventionsFeatureGate.ID(), false))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0MessagingConventionsFeatureGate.ID(), prevDontEmit))
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1MessagingConventionsFeatureGate.ID(), prevEmitV1))
	})

	_, err := GenerateTraces("testdata/generated_pict_pairs_traces.txt", "testdata/generated_pict_pairs_spans.txt")
	require.ErrorContains(t, err, "internal.coreinternal.goldendataset.DontEmitV0MessagingConventions cannot be enabled without enabling internal.coreinternal.goldendataset.EmitV1MessagingConventions")
}

func TestGenerateTracesInvalidDatabaseFeatureGateCombination(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0DatabaseConventionsFeatureGate.ID(), true))
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1DatabaseConventionsFeatureGate.ID(), false))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0DatabaseConventionsFeatureGate.ID(), false))
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1DatabaseConventionsFeatureGate.ID(), false))
	})

	_, err := GenerateTraces("testdata/generated_pict_pairs_traces.txt", "testdata/generated_pict_pairs_spans.txt")
	require.ErrorContains(t, err, "internal.coreinternal.goldendataset.DontEmitV0DatabaseConventions cannot be enabled without enabling internal.coreinternal.goldendataset.EmitV1DatabaseConventions")
}
