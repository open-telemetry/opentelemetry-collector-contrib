// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/transport"

import (
	"context"
	"sync"
)

// MockReporter provides a Reporter that provides some useful functionalities for
// tests (eg.: wait for certain number of messages).
type MockReporter struct {
	wgMetricsProcessed sync.WaitGroup
}

var _ Reporter = (*MockReporter)(nil)

// NewMockReporter returns a new instance of a MockReporter.
func NewMockReporter(expectedOnMetricsProcessedCalls int) *MockReporter {
	m := MockReporter{}
	m.wgMetricsProcessed.Add(expectedOnMetricsProcessedCalls)
	return &m
}

func (m *MockReporter) OnDataReceived(ctx context.Context) context.Context {
	return ctx
}

func (m *MockReporter) OnTranslationError(_ context.Context, _ error) {}

func (m *MockReporter) OnMetricsProcessed(_ context.Context, _ int, _ error) {
	m.wgMetricsProcessed.Done()
}

func (m *MockReporter) OnDebugf(_ string, _ ...interface{}) {
}

// WaitAllOnMetricsProcessedCalls blocks until the number of expected calls
// specified at creation of the reporter is completed.
func (m *MockReporter) WaitAllOnMetricsProcessedCalls() {
	m.wgMetricsProcessed.Wait()
}
