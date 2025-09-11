// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
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
		tsp, err := newTracesProcessor(t.Context(), set, sink, *cfg)
		if err != nil {
			t.Fatal(err)
		}
		_ = tsp.ConsumeTraces(t.Context(), traces)
	})
}
