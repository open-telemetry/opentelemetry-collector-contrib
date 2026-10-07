// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v3"
)

// TestReadmeConfigExamples round-trips every YAML config example in the README
// through confmap unmarshalling and Config.Validate, so documented examples
// cannot drift from the real config surface. Snippet-level blocks (a bare
// sampler or rule list) are wrapped in a minimal valid config first.
func TestReadmeConfigExamples(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	require.NoError(t, err)

	fence := regexp.MustCompile("(?s)```yaml\n(.*?)```")
	blocks := fence.FindAllStringSubmatch(string(readme), -1)
	require.NotEmpty(t, blocks)

	validated := 0
	for i, m := range blocks {
		body := m[1]
		var parsed any
		require.NoError(t, yaml.Unmarshal([]byte(body), &parsed), "README yaml block %d does not parse", i)

		cfgMap, ok := extractProcessorConfig(parsed)
		if !ok {
			continue // not a processor config example (e.g. tracestate or pipeline-only)
		}

		cfg := createDefaultConfig().(*Config)
		cm := confmap.NewFromStringMap(cfgMap)
		require.NoError(t, cm.Unmarshal(cfg), "README yaml block %d does not unmarshal:\n%s", i, body)
		require.NoError(t, cfg.Validate(), "README yaml block %d does not validate:\n%s", i, body)
		validated++
	}
	require.GreaterOrEqual(t, validated, 10, "expected the README to contain processor config examples")
}

// extractProcessorConfig normalizes the various example shapes in the README
// (full collector config, bare rules list, bare rule item, bare sampler block)
// into a adaptive_tail_sampling processor config map. Returns false for yaml blocks
// that are not processor configuration.
func extractProcessorConfig(parsed any) (map[string]any, bool) {
	defaults := func(rules any) map[string]any {
		return map[string]any{
			"trace_timeout":  "30s",
			"decision_delay": "2s",
			"num_traces":     1000,
			"rules":          rules,
		}
	}
	switch v := parsed.(type) {
	case map[string]any:
		if procs, ok := v["processors"].(map[string]any); ok {
			ds, ok := procs["adaptive_tail_sampling"].(map[string]any)
			return ds, ok
		}
		if rules, ok := v["rules"]; ok && len(v) == 1 {
			return defaults(rules), true
		}
		if sampler, ok := v["sampler"]; ok && len(v) == 1 {
			return defaults([]any{map[string]any{"name": "example", "sampler": sampler}}), true
		}
		return nil, false
	case []any:
		// A bare list of rule entries (each must look like a rule).
		for _, item := range v {
			rule, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			if _, hasName := rule["name"]; !hasName {
				return nil, false
			}
		}
		return defaults(v), true
	default:
		return nil, false
	}
}
