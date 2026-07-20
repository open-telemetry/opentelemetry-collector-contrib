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
