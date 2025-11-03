// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package examplesampler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailsampling/examplesampler/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestExtension(t *testing.T) {
	extension, err := NewExtension(extensiontest.NewNopSettings(metadata.Type), &Config{})
	require.NoError(t, err)

	defer func() {
		assert.NoError(t, extension.Shutdown(t.Context()))
	}()

	evaluator, err := extension.(*exampleExtension).NewEvaluator("example", nil)
	require.NoError(t, err)

	traceID := pcommon.NewTraceIDEmpty()
	traces := ptrace.NewTraces()
	td := &samplingpolicy.TraceData{
		ReceivedBatches: traces,
	}
	decision, err := evaluator.Evaluate(t.Context(), traceID, td)
	require.NoError(t, err)
	assert.Equal(t, samplingpolicy.NotSampled, decision)
}
