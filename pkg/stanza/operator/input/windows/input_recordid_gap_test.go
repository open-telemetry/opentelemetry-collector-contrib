// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// mockTelemetry records calls to RecordMissedEvents for assertion in tests.
type mockTelemetry struct {
	missedCalls []int64
}

func (*mockTelemetry) RecordEventSize(_ context.Context, _ string, _ int)     {}
func (*mockTelemetry) RecordChannelSize(_ context.Context, _ string, _ int64) {}
func (*mockTelemetry) RecordBatchSize(_ context.Context, _ string, _ int64)   {}
func (m *mockTelemetry) RecordMissedEvents(_ context.Context, _ string, count int64) {
	m.missedCalls = append(m.missedCalls, count)
}

func newTestInputWithTelemetry(mock *mockTelemetry) *Input {
	input := newInput(component.TelemetrySettings{Logger: zap.NewNop()})
	input.telemetry = mock
	return input
}

// TestCheckRecordIDGap_NoGap verifies that consecutive RecordIDs do not trigger a metric.
func TestCheckRecordIDGap_NoGap(t *testing.T) {
	mock := &mockTelemetry{}
	input := newTestInputWithTelemetry(mock)
	input.channel = "Application"
	input.lastRecordID = 100

	input.checkRecordIDGap(t.Context(), &EventXML{RecordID: 101})

	assert.Empty(t, mock.missedCalls)
	assert.Equal(t, uint64(101), input.lastRecordID)
}

// TestCheckRecordIDGap_GapDetected verifies that a non-contiguous RecordID emits the correct count.
func TestCheckRecordIDGap_GapDetected(t *testing.T) {
	mock := &mockTelemetry{}
	input := newTestInputWithTelemetry(mock)
	input.channel = "Application"
	input.lastRecordID = 100

	// RecordID jumps from 100 to 105 — 4 events are estimated missed.
	input.checkRecordIDGap(t.Context(), &EventXML{RecordID: 105})

	assert.Equal(t, []int64{4}, mock.missedCalls)
	assert.Equal(t, uint64(105), input.lastRecordID)
}

// TestCheckRecordIDGap_FirstEvent verifies that the first event (lastRecordID == 0) never triggers a gap.
func TestCheckRecordIDGap_FirstEvent(t *testing.T) {
	mock := &mockTelemetry{}
	input := newTestInputWithTelemetry(mock)
	input.channel = "Application"
	input.lastRecordID = 0

	input.checkRecordIDGap(t.Context(), &EventXML{RecordID: 500})

	assert.Empty(t, mock.missedCalls)
	assert.Equal(t, uint64(500), input.lastRecordID)
}

// TestCheckRecordIDGap_ZeroRecordID verifies that events with no RecordID are skipped.
func TestCheckRecordIDGap_ZeroRecordID(t *testing.T) {
	mock := &mockTelemetry{}
	input := newTestInputWithTelemetry(mock)
	input.channel = "Application"
	input.lastRecordID = 100

	input.checkRecordIDGap(t.Context(), &EventXML{RecordID: 0})

	assert.Empty(t, mock.missedCalls)
	// lastRecordID should not advance when RecordID is absent.
	assert.Equal(t, uint64(100), input.lastRecordID)
}

// TestCheckRecordIDGap_QueryMode verifies that gap detection is skipped when channel is empty (query mode).
func TestCheckRecordIDGap_QueryMode(t *testing.T) {
	mock := &mockTelemetry{}
	input := newTestInputWithTelemetry(mock)
	input.channel = "" // query mode
	input.lastRecordID = 100

	input.checkRecordIDGap(t.Context(), &EventXML{RecordID: 200})

	assert.Empty(t, mock.missedCalls)
}
