// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filelogreceiver

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver/internal/testutil"
)

func BenchmarkReadSingleStaticFile(b *testing.B) {
	testCases := []struct {
		numLines int
	}{
		{
			numLines: 0,
		},
		{
			numLines: 1,
		},
		{
			numLines: 10,
		},
		{
			numLines: 20,
		},
		{
			numLines: 100,
		},
		{
			numLines: 200,
		},
		{
			numLines: 1_000,
		},
		{
			numLines: 10_000,
		},
	}

	for _, tc := range testCases {
		b.Run(fmt.Sprintf("%d-lines", tc.numLines), func(b *testing.B) {
			benchmarkReadSingleStaticFile(b, tc.numLines)
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

	b.ResetTimer()
	for range b.N {
		rcvr, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
		require.NoError(b, err)
		require.NoError(b, rcvr.Start(context.Background(), componenttest.NewNopHost()))

		require.Eventually(b, expectNLogs(sink, numLines), 2*time.Second, 2*time.Microsecond)
		sink.Reset()

		require.NoError(b, rcvr.Shutdown(context.Background()))
	}
}
