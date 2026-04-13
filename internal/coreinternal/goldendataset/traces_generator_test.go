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
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0RPCConventionsFeatureGate.ID(), true))
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1RPCConventionsFeatureGate.ID(), false))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetDontEmitV0RPCConventionsFeatureGate.ID(), false))
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.InternalCoreinternalGoldendatasetEmitV1RPCConventionsFeatureGate.ID(), false))
	})

	_, err := GenerateTraces("testdata/generated_pict_pairs_traces.txt", "testdata/generated_pict_pairs_spans.txt")
	require.ErrorContains(t, err, "internal.coreinternal.goldendataset.DontEmitV0RPCConventions cannot be enabled without enabling internal.coreinternal.goldendataset.EmitV1RPCConventions")
}
