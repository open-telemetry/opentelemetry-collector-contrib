// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestOTTLDefaultErrorMode(t *testing.T) {
	gate := metadata.ProcessorTailsamplingprocessorDefaultErrorModeIgnoreFeatureGate
	previous := gate.IsEnabled()
	t.Cleanup(func() { require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), previous)) })

	for _, enabled := range []bool{false, true} {
		for _, mode := range []ottl.ErrorMode{"", ottl.PropagateError, ottl.IgnoreError, ottl.SilentError} {
			for _, scenario := range []string{"span_conditions", "span_event_conditions", "next_span", "next_span_event", "no_match"} {
				t.Run(fmt.Sprintf("enabled=%t/mode=%s/%s", enabled, mode, scenario), func(t *testing.T) {
					require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
					core, logs := observer.New(zap.WarnLevel)
					settings := componenttest.NewNopTelemetrySettings()
					settings.Logger = zap.New(core)
					conditions := []string{`Substring(attributes["value"], 0, 4) == "keep"`, `name == "keep"`}
					var spanConditions, eventConditions []string
					trace := newTraceWithSpansAttributes([]spanWithAttributes{{}})
					span := trace.ReceivedBatches.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
					switch scenario {
					case "span_conditions":
						spanConditions = conditions
						span.SetName("keep")
					case "span_event_conditions":
						eventConditions = conditions
						span.Events().At(0).SetName("keep")
					case "next_span":
						spanConditions = conditions[:1]
						trace.ReceivedBatches.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty().Attributes().PutStr("value", "keep")
					case "next_span_event":
						eventConditions = conditions[:1]
						span.Events().AppendEmpty().Attributes().PutStr("value", "keep")
					case "no_match":
						spanConditions = conditions
					}
					filter, err := NewOTTLConditionFilter(settings, spanConditions, eventConditions, mode)
					require.NoError(t, err)
					decision, err := filter.Evaluate(t.Context(), pcommon.TraceID{}, trace)
					propagate := mode == ottl.PropagateError || (mode == "" && !enabled)
					if propagate {
						require.Error(t, err)
						require.Equal(t, samplingpolicy.Error, decision)
					} else {
						require.NoError(t, err)
						want := samplingpolicy.Sampled
						if scenario == "no_match" {
							want = samplingpolicy.NotSampled
						}
						require.Equal(t, want, decision)
						if mode == ottl.SilentError {
							require.Zero(t, logs.Len())
						} else {
							require.Positive(t, logs.Len())
						}
					}
				})
			}
		}
	}
}
