// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package key_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/key"
)

var (
	traceID      = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	emptyTraceID = [16]byte{}
	spanID1      = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	spanID2      = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

func TestEnsureDifferentKeys(t *testing.T) {
	t.Parallel()

	partitioner := key.Randomized{}
	k := partitioner.Partition(nil)
	assert.NotEmpty(t, k, "Must have a string that has a value")
	assert.NotEqual(t, k, partitioner.Partition(nil), "Must have different string values")
}

func TestEnsureSameKeysForSpansOfSameTrace(t *testing.T) {
	t.Parallel()

	partitioner := key.TraceID{}
	k := partitioner.Partition(newTrace(traceID, spanID1))

	assert.NotEmpty(t, k, "Must have a string that has a value")
	assert.Equal(t, k, partitioner.Partition(newTrace(traceID, spanID2)), "Must have same string values")
}

func TestRandomKeyForEmptyTraceID(t *testing.T) {
	t.Parallel()

	partitioner := key.TraceID{}
	k := partitioner.Partition(newTrace(emptyTraceID, spanID1))

	assert.NotEmpty(t, k, "Must have a string that has a value")
	assert.NotEqual(t, k, partitioner.Partition(newTrace(emptyTraceID, spanID1)), "Must have different string values")
}

func TestRandomKeyForInvalidTrace(t *testing.T) {
	t.Parallel()

	partitioner := key.TraceID{}
	k := partitioner.Partition(nil)

	assert.NotEmpty(t, k, "Must have a string that has a value")
	assert.NotEqual(t, k, partitioner.Partition(nil), "Must have different string values")
}

func newTrace(traceID [16]byte, spanID [8]byte) ptrace.Traces {
	trace := ptrace.NewTraces()
	span := trace.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)

	return trace
}
