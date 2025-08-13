// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbyattrsprocessor

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor/internal/metadata"
)

func FuzzProcessTraces(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ju := &ptrace.JSONUnmarshaler{}
		traces, err := ju.UnmarshalTraces(data)
		if err != nil {
			return
		}
		gap, err := createGroupByAttrsProcessor(processortest.NewNopSettings(metadata.Type), []string{})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = gap.processTraces(context.Background(), traces)
	})
}

func FuzzProcessLogs(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ju := &plog.JSONUnmarshaler{}
		logs, err := ju.UnmarshalLogs(data)
		if err != nil {
			return
		}
		gap, err := createGroupByAttrsProcessor(processortest.NewNopSettings(metadata.Type), []string{})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = gap.processLogs(context.Background(), logs)
	})
}

func FuzzProcessMetrics(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ju := &pmetric.JSONUnmarshaler{}
		metrics, err := ju.UnmarshalMetrics(data)
		if err != nil {
			return
		}
		gap, err := createGroupByAttrsProcessor(processortest.NewNopSettings(metadata.Type), []string{})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = gap.processMetrics(context.Background(), metrics)
	})
}
