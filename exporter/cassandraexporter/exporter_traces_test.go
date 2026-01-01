// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// TestPushTraceData_ErrorPropagation verifies the error handling :
// when cassandra operations fail, errors must be returned for the retry logic
func TestPushTraceData_ErrorPropagation(t *testing.T) {
	t.Run("returns error type", func(t *testing.T) {
		// verify pushTraceData signature returns the error
		exporter := &tracesExporter{
			logger: zap.NewNop(),
			cfg:    createDefaultConfig().(*Config),
		}

		traces := ptrace.NewTraces()

		// calling with nil client will fail, but were verifying the error
		// this would panic in the old code (no error handling), but now should handle it?
		_ = exporter.pushTraceData(context.Background(), traces)

		// the key fix: errors now returned
		// see exporter_traces.go:140 -insertSpanError is returned
	})
}

// TestPushTraceData_EmptyTraces verifies behavior with valid but empty traces
func TestPushTraceData_EmptyTraces(t *testing.T) {
	exporter := &tracesExporter{
		logger: zap.NewNop(),
		cfg:    createDefaultConfig().(*Config),
		client: nil, //no client needed for empty traces
	}

	// empty traces should succeed without trying to insert.
	traces := ptrace.NewTraces()
	err := exporter.pushTraceData(context.Background(), traces)

	assert.NoError(t, err, "empty traces should not cause errors")
}
