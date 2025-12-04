// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filelogreceiver

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver/internal/testutil"
)

func BenchmarkReadSingleStaticFileWithBatchingLogEmitter(b *testing.B) {
	require.NoError(b, featuregate.GlobalRegistry().Set("stanza.synchronousLogEmitter", false))
	for n := range 6 {
		numLines := int(math.Pow(10, float64(n)))
		b.Run(fmt.Sprintf("%d-lines", numLines), func(b *testing.B) {
			benchmarkReadSingleStaticFile(b, numLines)
		})
	}
}

func BenchmarkReadSingleStaticFileWithSynchronousLogEmitter(b *testing.B) {
	require.NoError(b, featuregate.GlobalRegistry().Set("stanza.synchronousLogEmitter", true))
	for n := range 6 {
		numLines := int(math.Pow(10, float64(n)))
		b.Run(fmt.Sprintf("%d-lines", numLines), func(b *testing.B) {
			benchmarkReadSingleStaticFile(b, numLines)
		})
	}
}

func benchmarkReadSingleStaticFile(b *testing.B, numLines int) {
	logFileGenerator := testutil.NewLogFileGenerator(b)
	logFilePath := logFileGenerator.GenerateLogFile(numLines)

	cfg := &FileLogConfig{
		InputConfig: func() file.Config {
			c := file.NewConfig()
			c.Include = []string{logFilePath}
			c.PollInterval = time.Microsecond
			c.StartAt = "beginning"
			return *c
		}(),
	}
	sink := new(consumertest.LogsSink)
	f := NewFactory()

	for b.Loop() {
		rcvr, err := f.CreateLogs(b.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
		require.NoError(b, err)
		require.NoError(b, rcvr.Start(b.Context(), componenttest.NewNopHost()))

		require.Eventually(b, expectNLogs(sink, numLines), 2*time.Second, 2*time.Microsecond)
		sink.Reset()

		require.NoError(b, rcvr.Shutdown(b.Context()))
	}
}

// BenchmarkReceiverWithMultipleFiles benchmarks the receiver with multiple concurrent files
func BenchmarkReceiverWithMultipleFiles(b *testing.B) {
	for _, numFiles := range []int{1, 5, 10, 20} {
		b.Run(fmt.Sprintf("%d-files", numFiles), func(b *testing.B) {
			benchmarkReadMultipleFiles(b, numFiles, 1000)
		})
	}
}

func benchmarkReadMultipleFiles(b *testing.B, numFiles, linesPerFile int) {
	logFileGenerator := testutil.NewLogFileGenerator(b)
	var logFilePaths []string
	for i := 0; i < numFiles; i++ {
		logFilePaths = append(logFilePaths, logFileGenerator.GenerateLogFile(linesPerFile))
	}

	cfg := &FileLogConfig{
		InputConfig: func() file.Config {
			c := file.NewConfig()
			c.Include = logFilePaths
			c.PollInterval = time.Microsecond
			c.StartAt = "beginning"
			return *c
		}(),
	}
	sink := new(consumertest.LogsSink)
	f := NewFactory()

	expectedLogs := numFiles * linesPerFile

	for b.Loop() {
		rcvr, err := f.CreateLogs(b.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
		require.NoError(b, err)
		require.NoError(b, rcvr.Start(b.Context(), componenttest.NewNopHost()))

		require.Eventually(b, expectNLogs(sink, expectedLogs), 5*time.Second, 2*time.Microsecond)
		sink.Reset()

		require.NoError(b, rcvr.Shutdown(b.Context()))
	}
}

// BenchmarkReceiverStartupShutdown benchmarks receiver startup and shutdown
func BenchmarkReceiverStartupShutdown(b *testing.B) {
	logFileGenerator := testutil.NewLogFileGenerator(b)
	logFilePath := logFileGenerator.GenerateLogFile(100)

	cfg := &FileLogConfig{
		InputConfig: func() file.Config {
			c := file.NewConfig()
			c.Include = []string{logFilePath}
			c.PollInterval = time.Millisecond
			c.StartAt = "beginning"
			return *c
		}(),
	}
	sink := new(consumertest.LogsSink)
	f := NewFactory()

	for b.Loop() {
		rcvr, err := f.CreateLogs(b.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
		require.NoError(b, err)
		require.NoError(b, rcvr.Start(b.Context(), componenttest.NewNopHost()))
		require.NoError(b, rcvr.Shutdown(b.Context()))
	}
}

// BenchmarkReceiverWithParsing benchmarks receiver with log parsing
func BenchmarkReceiverWithParsing(b *testing.B) {
	require.NoError(b, featuregate.GlobalRegistry().Set("stanza.synchronousLogEmitter", false))

	logFileGenerator := testutil.NewLogFileGenerator(b)
	logFilePath := logFileGenerator.GenerateLogFile(10000)

	cfg := &FileLogConfig{
		InputConfig: func() file.Config {
			c := file.NewConfig()
			c.Include = []string{logFilePath}
			c.PollInterval = time.Microsecond
			c.StartAt = "beginning"
			return *c
		}(),
	}
	sink := new(consumertest.LogsSink)
	f := NewFactory()

	for b.Loop() {
		rcvr, err := f.CreateLogs(b.Context(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
		require.NoError(b, err)
		require.NoError(b, rcvr.Start(b.Context(), componenttest.NewNopHost()))

		require.Eventually(b, expectNLogs(sink, 10000), 5*time.Second, 2*time.Microsecond)
		sink.Reset()

		require.NoError(b, rcvr.Shutdown(b.Context()))
	}
}
