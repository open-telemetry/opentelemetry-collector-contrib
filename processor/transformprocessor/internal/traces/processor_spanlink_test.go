// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

func setExperimentalGate(t *testing.T, enabled bool) {
	gate := metadata.ProcessorTransformEnableExperimentalFunctionsFeatureGate
	original := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), original))
	})
}

// constructSpanLinkTraces has one span with two distinguishable links and one span without links.
func constructSpanLinkTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	withLinks := ss.Spans().AppendEmpty()
	withLinks.SetName("withLinks")
	pubsub := withLinks.Links().AppendEmpty()
	pubsub.Attributes().PutStr("messaging.system", "pubsub")
	kafka := withLinks.Links().AppendEmpty()
	kafka.Attributes().PutStr("messaging.system", "kafka")
	ss.Spans().AppendEmpty().SetName("noLinks")
	return td
}

func newSpanLinkProcessor(cs common.ContextStatements) (*Processor, error) {
	return NewProcessor([]common.ContextStatements{cs}, ottl.IgnoreError, componenttest.NewNopTelemetrySettings(), DefaultSpanFunctions, DefaultSpanEventFunctions)
}

func Test_ProcessTraces_SpanLinkContext(t *testing.T) {
	setExperimentalGate(t, true)

	tests := []struct {
		name string
		cs   common.ContextStatements
		// checked is the messaging.system of each link expected to be marked, in order.
		checked []string
	}{
		{
			name: "explicit context",
			cs: common.ContextStatements{
				Context:    common.SpanLink,
				Statements: []string{`set(attributes["checked"], true) where attributes["messaging.system"] == "pubsub"`},
			},
			checked: []string{"pubsub"},
		},
		{
			name: "inferred context",
			cs: common.ContextStatements{
				Statements: []string{`set(spanlink.attributes["checked"], true) where spanlink.attributes["messaging.system"] == "pubsub"`},
			},
			checked: []string{"pubsub"},
		},
		{
			name: "conditions met",
			cs: common.ContextStatements{
				Context:    common.SpanLink,
				Conditions: []string{`span.name == "withLinks"`},
				Statements: []string{`set(attributes["checked"], true)`},
			},
			checked: []string{"pubsub", "kafka"},
		},
		{
			name: "conditions not met",
			cs: common.ContextStatements{
				Context:    common.SpanLink,
				Conditions: []string{`span.name == "noLinks"`},
				Statements: []string{`set(attributes["checked"], true)`},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := constructSpanLinkTraces()
			processor, err := newSpanLinkProcessor(tt.cs)
			require.NoError(t, err)

			_, err = processor.ProcessTraces(t.Context(), td)
			require.NoError(t, err)

			want := constructSpanLinkTraces()
			links := want.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Links()
			for i := range links.Len() {
				system, _ := links.At(i).Attributes().Get("messaging.system")
				for _, c := range tt.checked {
					if system.Str() == c {
						links.At(i).Attributes().PutBool("checked", true)
					}
				}
			}
			assert.Equal(t, want, td)
		})
	}
}

func Test_ProcessTraces_SpanLinkContextGate(t *testing.T) {
	cs := common.ContextStatements{Context: common.SpanLink, Statements: []string{`set(attributes["checked"], true)`}}

	setExperimentalGate(t, false)
	_, err := newSpanLinkProcessor(cs)
	require.ErrorContains(t, err, metadata.ProcessorTransformEnableExperimentalFunctionsFeatureGate.ID())

	setExperimentalGate(t, true)
	_, err = newSpanLinkProcessor(cs)
	require.NoError(t, err)
}
