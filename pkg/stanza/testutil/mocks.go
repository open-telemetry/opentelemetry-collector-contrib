// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"

import (
	context "context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// NewMockOperator will return a basic operator mock
func NewMockOperator(id string) *Operator {
	mockOutput := &Operator{}
	mockOutput.On("ID").Return(id)
	mockOutput.On("CanProcess").Return(true)
	mockOutput.On("CanOutput").Return(true)
	return mockOutput
}

// FakeOutput is an empty output used primarily for testing
type FakeOutput struct {
	Received         chan *entry.Entry
	logger           *zap.Logger
	processWithError bool
}

// NewFakeOutput creates a new fake output with default settings
func NewFakeOutput(tb testing.TB) *FakeOutput {
	return &FakeOutput{
		Received: make(chan *entry.Entry, 100),
		logger:   zaptest.NewLogger(tb),
	}
}

// NewFakeOutputWithProcessError creates a new fake output with default settings, which returns error on Process
func NewFakeOutputWithProcessError(tb testing.TB) *FakeOutput {
	return &FakeOutput{
		Received:         make(chan *entry.Entry, 100),
		logger:           zaptest.NewLogger(tb),
		processWithError: true,
	}
}

// CanOutput always returns false for a fake output
func (f *FakeOutput) CanOutput() bool { return false }

// CanProcess always returns true for a fake output
func (f *FakeOutput) CanProcess() bool { return true }

// ID always returns `fake` as the ID of a fake output operator
func (f *FakeOutput) ID() string { return "fake" }

// Logger returns the logger of a fake output
func (f *FakeOutput) Logger() *zap.Logger { return f.logger }

// Outputs always returns nil for a fake output
func (f *FakeOutput) Outputs() []operator.Operator { return nil }

// Outputs always returns nil for a fake output
func (f *FakeOutput) GetOutputIDs() []string { return nil }

// SetOutputs immediately returns nil for a fake output
func (f *FakeOutput) SetOutputs(_ []operator.Operator) error { return nil }

// SetOutputIDs immediately returns nil for a fake output
func (f *FakeOutput) SetOutputIDs(_ []string) {}

// Start immediately returns nil for a fake output
func (f *FakeOutput) Start(_ operator.Persister) error { return nil }

// Stop immediately returns nil for a fake output
func (f *FakeOutput) Stop() error { return nil }

// Type always return `fake_output` for a fake output
func (f *FakeOutput) Type() string { return "fake_output" }

func (f *FakeOutput) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	var errs error
	for i := range entries {
		errs = multierr.Append(errs, f.Process(ctx, entries[i]))
	}
	return errs
}

// Process will place all incoming entries on the Received channel of a fake output
func (f *FakeOutput) Process(_ context.Context, entry *entry.Entry) error {
	f.Received <- entry
	if f.processWithError {
		return errors.NewError("Operator can not process logs.", "")
	}
	return nil
}

// ExpectBody expects that a body will be received by the fake operator within a second
// and that it is equal to the given body
func (f *FakeOutput) ExpectBody(tb testing.TB, body any) {
	select {
	case e := <-f.Received:
		require.Equal(tb, body, e.Body)
	case <-time.After(time.Second):
		require.FailNowf(tb, "Timed out waiting for entry", "%s", body)
	}
}

// ExpectEntry expects that an entry will be received by the fake operator within a second
// and that it is equal to the given body
func (f *FakeOutput) ExpectEntry(tb testing.TB, expected *entry.Entry) {
	select {
	case e := <-f.Received:
		require.Equal(tb, expected, e)
	case <-time.After(time.Second):
		require.FailNowf(tb, "Timed out waiting for entry", "%v", expected)
	}
}

// ExpectEntries expects that the given entries will be received in any order
func (f *FakeOutput) ExpectEntries(tb testing.TB, expected []*entry.Entry) {
	entries := make([]*entry.Entry, 0, len(expected))
	for i := 0; i < len(expected); i++ {
		select {
		case e := <-f.Received:
			entries = append(entries, e)
		case <-time.After(time.Second):
			require.Fail(tb, "Timed out waiting for entry")
		}
		if tb.Failed() {
			break
		}
	}
	require.ElementsMatch(tb, expected, entries)
}

// ExpectNoEntry expects that no entry will be received within the specified time
func (f *FakeOutput) ExpectNoEntry(tb testing.TB, timeout time.Duration) {
	select {
	case <-f.Received:
		require.FailNow(tb, "Should not have received entry")
	case <-time.After(timeout):
		return
	}
}
