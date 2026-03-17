// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestFillBufferUntilChanEmptyCollectsACKChannels(t *testing.T) {
	eventCh := make(chan eventWithACK, 3)

	// Create three events: two with ackCh, one without.
	ackCh1 := make(chan error, 1)
	ackCh2 := make(chan error, 1)

	lr1 := plog.NewLogRecordSlice()
	lr1.AppendEmpty().Body().SetStr("log1")

	lr2 := plog.NewLogRecordSlice()
	lr2.AppendEmpty().Body().SetStr("log2")

	lr3 := plog.NewLogRecordSlice()
	lr3.AppendEmpty().Body().SetStr("log3")

	eventCh <- eventWithACK{event: &testEvent{logs: lr1}, ackCh: ackCh1}
	eventCh <- eventWithACK{event: &testEvent{logs: lr2}, ackCh: nil}
	eventCh <- eventWithACK{event: &testEvent{logs: lr3}, ackCh: ackCh2}

	c := &collector{eventCh: eventCh}
	dest := plog.NewLogRecordSlice()
	var ackChs []chan error

	ackChs = c.fillBufferUntilChanEmpty(dest, ackChs)

	// All three events' log records should be merged.
	require.Equal(t, 3, dest.Len())

	// Only the two non-nil ackCh channels should be collected.
	require.Len(t, ackChs, 2)
	assert.Equal(t, ackCh1, ackChs[0])
	assert.Equal(t, ackCh2, ackChs[1])
}

// testEvent is a minimal event implementation for unit tests.
type testEvent struct {
	logs plog.LogRecordSlice
}

func (*testEvent) DecodeMsg(_ *msgp.Reader) error {
	return nil
}

func (e *testEvent) LogRecords() plog.LogRecordSlice {
	return e.logs
}

func (*testEvent) Chunk() string {
	return ""
}

func (*testEvent) Compressed() string {
	return ""
}
