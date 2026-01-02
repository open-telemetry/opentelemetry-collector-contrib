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

// TestPushTraceData_EmptyTraces verifies behavior with valid but empty traces
func TestPushTraceData_EmptyTraces(t *testing.T) {
	exporter := &tracesExporter{
		logger: zap.NewNop(),
		cfg:    createDefaultConfig().(*Config),
		client: nil, // no client needed for empty traces
	}

	// empty traces should succeed without trying to insert
	traces := ptrace.NewTraces()
	err := exporter.pushTraceData(context.Background(), traces)

	assert.NoError(t, err, "empty traces should not cause errors")
}
