// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor/internal/metadata"
)

func FuzzConsumeTraces(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ju := &ptrace.JSONUnmarshaler{}
		traces, err := ju.UnmarshalTraces(data)
		if err != nil {
			return
		}
		sink := new(consumertest.TracesSink)
		set := processortest.NewNopSettings(metadata.Type)
		cfg := &Config{}
		tsp, err := newTracesProcessor(context.Background(), set, cfg, sink)
		if err != nil {
			t.Fatal(err)
		}
		_ = tsp.ConsumeTraces(context.Background(), traces)
	})
}

func FuzzConsumeLogs(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ju := &plog.JSONUnmarshaler{}
		logs, err := ju.UnmarshalLogs(data)
		if err != nil {
			return
		}
		nextConsumer := consumertest.NewNop()
		cfg := &Config{}
		lp, err := newLogsProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
		if err != nil {
			t.Fatal(err)
		}
		_ = lp.ConsumeLogs(context.Background(), logs)
	})
}
