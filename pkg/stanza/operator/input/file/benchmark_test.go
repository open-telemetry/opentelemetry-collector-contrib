// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/filetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func BenchmarkReadExistingLogs(b *testing.B) {
	for n := range 6 { // powers of 10
		numLines := int(math.Pow(10, float64(n)))
		b.Run(fmt.Sprintf("%d-lines", numLines), func(b *testing.B) {
			benchmarkReadExistingLogs(b, numLines)
		})
	}
}

func benchmarkReadExistingLogs(b *testing.B, lines int) {
	logFilePath := generateLogFile(b, lines)

	config := NewConfig()
	config.Include = []string{
		logFilePath,
	}
	// Use aggressive poll interval so we're not measuring excess sleep time
	config.PollInterval = time.Microsecond
	config.StartAt = "beginning"

	op, err := config.Build(componenttest.NewNopTelemetrySettings())
	require.NoError(b, err)
	op.SetOutputIDs([]string{benchmarkOutputID})

	doneChan := make(chan struct{})
	require.NoError(b, op.SetOutputs([]operator.Operator{newBenchmarkOutput(lines, doneChan)}))

	for b.Loop() {
		require.NoError(b, op.Start(testutil.NewUnscopedMockPersister()))

		// Wait until all logs are read
		if lines > 0 {
			<-doneChan
		}
		require.NoError(b, op.Stop())
	}
}

func generateLogFile(tb testing.TB, numLines int) string {
	f := filetest.OpenTemp(tb, tb.TempDir())
	for range numLines {
		_, err := f.Write(filetest.TokenWithLength(999))
		require.NoError(tb, err)
		_, err = f.WriteString("\n")
		require.NoError(tb, err)
	}
	return f.Name()
}

type benchmarkOutput struct {
	totalLogs    int
	logsReceived int
	doneChan     chan<- struct{}
}

// Assert that benchmarkOutput implements the operator.Operator interface
var _ operator.Operator = (*benchmarkOutput)(nil)

func newBenchmarkOutput(totalLogs int, doneChan chan<- struct{}) *benchmarkOutput {
	return &benchmarkOutput{
		totalLogs: totalLogs,
		doneChan:  doneChan,
	}
}

// CanOutput implements operator.Operator.
func (*benchmarkOutput) CanOutput() bool { return false }

// CanProcess implements operator.Operator.
func (*benchmarkOutput) CanProcess() bool { return true }

// GetOutputIDs implements operator.Operator.
func (*benchmarkOutput) GetOutputIDs() []string { return nil }

const benchmarkOutputID = "benchmark"

// ID implements operator.Operator.
func (*benchmarkOutput) ID() string { return benchmarkOutputID }

// Logger implements operator.Operator.
func (*benchmarkOutput) Logger() *zap.Logger { return nil }

// Outputs implements operator.Operator.
func (*benchmarkOutput) Outputs() []operator.Operator { return nil }

// SetOutputIDs implements operator.Operator.
func (*benchmarkOutput) SetOutputIDs([]string) {}

// SetOutputs implements operator.Operator.
func (*benchmarkOutput) SetOutputs([]operator.Operator) error { return nil }

// Start implements operator.Operator.
func (*benchmarkOutput) Start(operator.Persister) error {
	return nil
}

// Stop implements operator.Operator.
func (*benchmarkOutput) Stop() error {
	return nil
}

// Type implements operator.Operator.
func (*benchmarkOutput) Type() string { return "benchmark_output" }

func (o *benchmarkOutput) Process(_ context.Context, _ *entry.Entry) error {
	o.logsReceived++
	if o.logsReceived == o.totalLogs {
		o.doneChan <- struct{}{}
		o.logsReceived = 0
	}
	return nil
}

func (o *benchmarkOutput) ProcessBatch(_ context.Context, entries []*entry.Entry) error {
	o.logsReceived += len(entries)
	if o.logsReceived >= o.totalLogs {
		o.doneChan <- struct{}{}
		o.logsReceived = 0
	}
	return nil
}
