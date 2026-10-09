// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestOTTLDefaultErrorModeFromConfig(t *testing.T) {
	gate := metadata.ProcessorTailsamplingprocessorDefaultErrorModeIgnoreFeatureGate
	previous := gate.IsEnabled()
	t.Cleanup(func() { require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), previous)) })

	for _, enabled := range []bool{false, true} {
		for _, mode := range []string{"", "propagate", "ignore", "silent"} {
			for _, parent := range []string{"top_level", "and", "composite", "drop"} {
				t.Run(fmt.Sprintf("enabled=%t/mode=%s/%s", enabled, mode, parent), func(t *testing.T) {
					require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
					ottlConfig := map[string]any{"span": []any{`Substring(attributes["missing"], 0, 4) == "keep"`, `name == "keep"`}}
					if mode != "" {
						ottlConfig["error_mode"] = mode
					}
					policy := map[string]any{"name": "ottl", "type": "ottl_condition", "ottl_condition": ottlConfig}
					switch parent {
					case "and", "drop":
						policy = map[string]any{"name": "parent", "type": parent, parent: map[string]any{parent + "_sub_policy": []any{policy}}}
					case "composite":
						policy = map[string]any{"name": "parent", "type": parent, parent: map[string]any{
							"max_total_spans_per_second": 100,
							"policy_order":               []any{"ottl"},
							"composite_sub_policy":       []any{policy},
							"rate_allocation":            []any{map[string]any{"policy": "ottl", "percent": 100}},
						}}
					}
					cfg := NewFactory().CreateDefaultConfig().(*Config)
					require.NoError(t, confmap.NewFromStringMap(map[string]any{"policies": []any{policy}}).Unmarshal(cfg))
					require.NoError(t, cfg.Validate())
					evaluator, err := getPolicyEvaluator(componenttest.NewNopTelemetrySettings(), &cfg.PolicyCfgs[0], nil)
					require.NoError(t, err)
					traces := ptrace.NewTraces()
					traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("keep")
					decision, err := evaluator.Evaluate(t.Context(), pcommon.TraceID{}, &samplingpolicy.TraceData{ReceivedBatches: traces, SpanCount: 1})
					if mode == "propagate" || (mode == "" && !enabled) {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
						want := samplingpolicy.Sampled
						if parent == "drop" {
							want = samplingpolicy.Dropped
						}
						require.Equal(t, want, decision)
					}
				})
			}
		}
	}
}
