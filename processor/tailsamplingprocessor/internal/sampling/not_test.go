// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

// mockEvaluator is a mock implementation of samplingpolicy.Evaluator for testing
type mockEvaluator struct {
	decision samplingpolicy.Decision
	err      error
}

func (m *mockEvaluator) Evaluate(_ context.Context, _ pcommon.TraceID, _ *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	return m.decision, m.err
}

func TestNotSamplingPolicy_Evaluate_Sampled(t *testing.T) {
	logger := zap.NewNop()
	mockSubPolicy := &mockEvaluator{decision: samplingpolicy.Sampled}

	notPolicy := NewNot(logger, mockSubPolicy)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	traceData := &samplingpolicy.TraceData{}

	decision, err := notPolicy.Evaluate(t.Context(), traceID, traceData)

	assert.NoError(t, err)
	assert.Equal(t, samplingpolicy.NotSampled, decision)
}

func TestNotSamplingPolicy_Evaluate_NotSampled(t *testing.T) {
	logger := zap.NewNop()
	mockSubPolicy := &mockEvaluator{decision: samplingpolicy.NotSampled}

	notPolicy := NewNot(logger, mockSubPolicy)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	traceData := &samplingpolicy.TraceData{}

	decision, err := notPolicy.Evaluate(t.Context(), traceID, traceData)

	assert.NoError(t, err)
	assert.Equal(t, samplingpolicy.Sampled, decision)
}

func TestNotSamplingPolicy_Evaluate_OtherDecision(t *testing.T) {
	logger := zap.NewNop()
	// Using a decision value that should be handled by the default case
	mockSubPolicy := &mockEvaluator{decision: samplingpolicy.Unspecified}

	notPolicy := NewNot(logger, mockSubPolicy)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	traceData := &samplingpolicy.TraceData{}

	decision, err := notPolicy.Evaluate(t.Context(), traceID, traceData)

	assert.NoError(t, err)
	assert.Equal(t, samplingpolicy.Unspecified, decision)
}

func TestNotSamplingPolicy_Evaluate_Err_Not_Nil(t *testing.T) {
	logger := zap.NewNop()
	expectedError := assert.AnError
	mockSubPolicy := &mockEvaluator{decision: samplingpolicy.Error, err: expectedError}

	notPolicy := NewNot(logger, mockSubPolicy)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	traceData := &samplingpolicy.TraceData{}

	decision, err := notPolicy.Evaluate(t.Context(), traceID, traceData)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	assert.Equal(t, samplingpolicy.Error, decision)
}
