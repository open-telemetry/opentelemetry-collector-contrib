// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package severity

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

type severityTestCase struct {
	name       string
	sample     any
	mappingSet string
	mapping    map[string]any
	buildErr   bool
	parseErr   bool
	expected   entry.Severity
}

func TestSeverityParser(t *testing.T) {
	allTheThingsMap := map[string]any{
		"info":   "3xx",
		"error3": "4xx",
		"debug4": "5xx",
		"trace2": []any{
			"ttttttracer",
			[]byte{100, 100, 100},
			map[string]any{"min": 1111, "max": 1234},
		},
		"fatal2": "",
	}

	testCases := []severityTestCase{
		{
			name:     "unknown",
			sample:   "blah",
			mapping:  nil,
			expected: entry.Default,
		},
		{
			name:     "error",
			sample:   "error",
			mapping:  nil,
			expected: entry.Error,
		},
		{
			name:     "error-capitalized",
			sample:   "Error",
			mapping:  nil,
			expected: entry.Error,
		},
		{
			name:     "error-all-caps",
			sample:   "ERROR",
			mapping:  nil,
			expected: entry.Error,
		},
		{
			name:     "custom-string",
			sample:   "NOOOOOOO",
			mapping:  map[string]any{"error": "NOOOOOOO"},
			expected: entry.Error,
		},
		{
			name:     "custom-string-caps-key",
			sample:   "NOOOOOOO",
			mapping:  map[string]any{"ErRoR": "NOOOOOOO"},
			expected: entry.Error,
		},
		{
			name:     "custom-int",
			sample:   1234,
			mapping:  map[string]any{"error": 1234},
			expected: entry.Error,
		},
		{
			name:     "mixed-list-string",
			sample:   "ThiS Is BaD",
			mapping:  map[string]any{"error": []any{"NOOOOOOO", "this is bad", 1234}},
			expected: entry.Error,
		},
		{
			name:     "mixed-list-int",
			sample:   1234,
			mapping:  map[string]any{"error": []any{"NOOOOOOO", "this is bad", 1234}},
			expected: entry.Error,
		},
		{
			name:     "in-range",
			sample:   123,
			mapping:  map[string]any{"error": map[string]any{"min": 120, "max": 125}},
			expected: entry.Error,
		},
		{
			name:     "in-range-min",
			sample:   120,
			mapping:  map[string]any{"error": map[string]any{"min": 120, "max": 125}},
			expected: entry.Error,
		},
		{
			name:     "in-range-max",
			sample:   125,
			mapping:  map[string]any{"error": map[string]any{"min": 120, "max": 125}},
			expected: entry.Error,
		},
		{
			name:     "out-of-range-min-minus",
			sample:   119,
			mapping:  map[string]any{"error": map[string]any{"min": 120, "max": 125}},
			expected: entry.Default,
		},
		{
			name:     "out-of-range-max-plus",
			sample:   126,
			mapping:  map[string]any{"error": map[string]any{"min": 120, "max": 125}},
			expected: entry.Default,
		},
		{
			name:     "range-out-of-order",
			sample:   123,
			mapping:  map[string]any{"error": map[string]any{"min": 125, "max": 120}},
			expected: entry.Error,
		},
		{
			name:     "Http2xx-hit",
			sample:   201,
			mapping:  map[string]any{"error": "2xx"},
			expected: entry.Error,
		},
		{
			name:     "Http2xx-miss",
			sample:   301,
			mapping:  map[string]any{"error": "2xx"},
			expected: entry.Default,
		},
		{
			name:     "Http3xx-hit",
			sample:   301,
			mapping:  map[string]any{"error": "3xx"},
			expected: entry.Error,
		},
		{
			name:     "Http4xx-hit",
			sample:   "404",
			mapping:  map[string]any{"error": "4xx"},
			expected: entry.Error,
		},
		{
			name:     "Http5xx-hit",
			sample:   555,
			mapping:  map[string]any{"error": "5xx"},
			expected: entry.Error,
		},
		{
			name:     "Http-All",
			sample:   "301",
			mapping:  map[string]any{"debug": "2xx", "info": "3xx", "error": "4xx", "warn": "5xx"},
			expected: entry.Info,
		},
		{
			name:     "all-the-things-midrange",
			sample:   1234,
			mapping:  allTheThingsMap,
			expected: entry.Trace2,
		},
		{
			name:     "all-the-things-bytes",
			sample:   []byte{100, 100, 100},
			mapping:  allTheThingsMap,
			expected: entry.Trace2,
		},
		{
			name:     "all-the-things-empty",
			sample:   "",
			mapping:  allTheThingsMap,
			expected: entry.Fatal2,
		},
		{
			name:     "all-the-things-3xx",
			sample:   "399",
			mapping:  allTheThingsMap,
			expected: entry.Info,
		},
		{
			name:     "all-the-things-miss",
			sample:   "miss",
			mapping:  allTheThingsMap,
			expected: entry.Default,
		},
		{
			name:       "base-mapping-none",
			sample:     "error",
			mappingSet: "none",
			mapping:    nil,
			expected:   entry.Default, // not error
		},
	}

	rootField := entry.NewBodyField()
	someField := entry.NewBodyField("some_field")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootCfg := parseSeverityTestConfig(rootField, tc.mappingSet, tc.mapping)
			rootEntry := makeTestEntry(t, rootField, tc.sample)
			t.Run("root", runSeverityParseTest(rootCfg, rootEntry, tc.buildErr, tc.parseErr, tc.expected))

			nonRootCfg := parseSeverityTestConfig(someField, tc.mappingSet, tc.mapping)
			nonRootEntry := makeTestEntry(t, someField, tc.sample)
			t.Run("non-root", runSeverityParseTest(nonRootCfg, nonRootEntry, tc.buildErr, tc.parseErr, tc.expected))
		})
	}
}

func runSeverityParseTest(cfg *Config, ent *entry.Entry, buildErr bool, parseErr bool, expected entry.Severity) func(*testing.T) {
	return func(t *testing.T) {
		op, err := cfg.Build(testutil.Logger(t))
		if buildErr {
			require.Error(t, err, "expected error when configuring operator")
			return
		}
		require.NoError(t, err, "unexpected error when configuring operator")

		mockOutput := &testutil.Operator{}
		resultChan := make(chan *entry.Entry, 1)
		mockOutput.On("Process", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			resultChan <- args.Get(1).(*entry.Entry)
		}).Return(nil)

		severityParser := op.(*Parser)
		severityParser.OutputOperators = []operator.Operator{mockOutput}

		err = severityParser.Parse(ent)
		if parseErr {
			require.Error(t, err, "expected error when parsing sample")
			return
		}
		require.NoError(t, err)

		require.Equal(t, expected, ent.Severity)
	}
}

func parseSeverityTestConfig(parseFrom entry.Field, preset string, mapping map[string]any) *Config {
	cfg := NewConfigWithID("test_operator_id")
	cfg.OutputIDs = []string{"output1"}
	cfg.SeverityConfig = helper.SeverityConfig{
		ParseFrom: &parseFrom,
		Preset:    preset,
		Mapping:   mapping,
	}
	return cfg
}

func makeTestEntry(t *testing.T, field entry.Field, value any) *entry.Entry {
	e := entry.New()
	require.NoError(t, e.Set(field, value))
	return e
}
