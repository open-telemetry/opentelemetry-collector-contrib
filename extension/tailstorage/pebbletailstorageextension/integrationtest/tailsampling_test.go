// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailstorage/pebbletailstorageextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
)

// tailStorageFeatureGateID is duplicated for tests in this package only.
//
// TODO: Use the tailsamplingprocessor tail storage feature gate ID once it is public.
const tailStorageFeatureGateID = "processor.tailsamplingprocessor.tailstorageextension"

func TestTailSamplingWithPebbleTailStorageExtension(t *testing.T) {
	enableTailStorageFeatureGateForBenchmark(t)

	extFactory := pebbletailstorageextension.NewFactory()
	extID := component.NewIDWithName(metadata.Type, "e2e")

	extCfg := extFactory.CreateDefaultConfig().(*pebbletailstorageextension.Config)
	extCfg.Directory = t.TempDir()

	ext, err := extFactory.Create(t.Context(), extensiontest.NewNopSettings(metadata.Type), extCfg)
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, ext.Shutdown(t.Context()))
	})

	processorFactory := tailsamplingprocessor.NewFactory()
	processorCfg := processorFactory.CreateDefaultConfig().(*tailsamplingprocessor.Config)
	processorCfg.DecisionWait = 10 * time.Millisecond
	processorCfg.NumTraces = 10
	processorCfg.TailStorageID = &extID
	require.NoError(t, confmap.NewFromStringMap(map[string]any{
		"policies": []any{
			map[string]any{
				"name": "always-sample",
				"type": string(tailsamplingprocessor.AlwaysSample),
			},
		},
	}).Unmarshal(processorCfg))

	sink := new(consumertest.TracesSink)
	proc, err := processorFactory.CreateTraces(
		t.Context(),
		processortest.NewNopSettings(processorFactory.Type()),
		processorCfg,
		sink,
	)
	require.NoError(t, err)

	host := e2eHost{
		extensions: map[component.ID]component.Component{
			extID: ext,
		},
	}
	require.NoError(t, proc.Start(t.Context(), host))
	t.Cleanup(func() {
		require.NoError(t, proc.Shutdown(t.Context()))
	})

	require.NoError(t, proc.ConsumeTraces(t.Context(), singleTrace()))
	require.Eventually(t, func() bool {
		return len(sink.AllTraces()) > 0
	}, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, 1, sink.AllTraces()[0].SpanCount())
}

func singleTrace() ptrace.Traces {
	td := ptrace.NewTraces()
	traceID := pcommon.TraceID([16]byte{0x10, 0x20, 0x30, 0x40})
	spanID := pcommon.SpanID([8]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01, 0x02, 0x03})

	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetName("e2e-span")
	return td
}

type e2eHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (h e2eHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}
