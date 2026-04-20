// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/pdata/plog"
)

// fakeEvent is a minimal event implementation for collector unit tests.
type fakeEvent struct {
	records plog.LogRecordSlice
}

func (*fakeEvent) DecodeMsg(_ *msgp.Reader) error    { return nil }
func (f *fakeEvent) LogRecords() plog.LogRecordSlice { return f.records }
func (*fakeEvent) Chunk() string                     { return "" }
func (*fakeEvent) Compressed() string                { return "" }

func newFakeEvent(numRecords int) event {
	records := plog.NewLogRecordSlice()
	for range numRecords {
		records.AppendEmpty()
	}
	return &fakeEvent{records: records}
}

// TestAppendEventToLogsIsolation verifies that each call to appendEventToLogs
// creates a distinct ResourceLogs, so resource attributes set by downstream
// processors are scoped to a single event only.
func TestAppendEventToLogsIsolation(t *testing.T) {
	out := plog.NewLogs()

	appendEventToLogs(out, newFakeEvent(2))
	appendEventToLogs(out, newFakeEvent(3))

	require.Equal(t, 2, out.ResourceLogs().Len())
	require.Equal(t, 2, out.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, 3, out.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().Len())
}

// TestFillBufferUntilChanEmptyIsolation verifies that draining multiple events
// from the channel produces one ResourceLogs per event rather than merging all
// records into a shared ResourceLogs.
func TestFillBufferUntilChanEmptyIsolation(t *testing.T) {
	const N = 5
	eventCh := make(chan event, N)
	for range N {
		eventCh <- newFakeEvent(1)
	}

	c := &collector{eventCh: eventCh}
	out := plog.NewLogs()
	c.fillBufferUntilChanEmpty(out)

	require.Equal(t, N, out.ResourceLogs().Len())
	for i := range N {
		require.Equal(t, 1, out.ResourceLogs().At(i).ScopeLogs().At(0).LogRecords().Len(),
			"event %d should be in its own ResourceLogs", i)
	}
}
