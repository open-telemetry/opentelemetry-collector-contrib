// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonparser

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newTestParser(t *testing.T) *Parser {
	config := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := config.Build(set)
	require.NoError(t, err)
	return op.(*Parser)
}

func TestConfigBuild(t *testing.T) {
	config := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := config.Build(set)
	require.NoError(t, err)
	require.IsType(t, &Parser{}, op)
}

func TestConfigBuildFailure(t *testing.T) {
	config := NewConfigWithID("test")
	config.OnError = "invalid_on_error"
	set := componenttest.NewNopTelemetrySettings()
	_, err := config.Build(set)
	require.ErrorContains(t, err, "invalid `on_error` field")
}

func TestParserStringFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("invalid")
	require.ErrorContains(t, err, "expected { character for map value")
}

func TestParserByteFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]byte("invalid"))
	require.ErrorContains(t, err, "type []uint8 cannot be parsed as JSON")
}

func TestParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.ErrorContains(t, err, "type []int cannot be parsed as JSON")
}

func TestJSONImplementations(t *testing.T) {
	require.Implements(t, (*operator.Operator)(nil), new(Parser))
}

func TestParser(t *testing.T) {
	cases := []struct {
		name      string
		configure func(*Config)
		input     *entry.Entry
		expect    *entry.Entry
	}{
		{
			"simple",
			func(_ *Config) {},
			&entry.Entry{
				Body: `{}`,
			},
			&entry.Entry{
				Attributes: map[string]any{},
				Body:       `{}`,
			},
		},
		{
			"nested",
			func(_ *Config) {},
			&entry.Entry{
				Body: `{"superkey":"superval"}`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"superkey": "superval",
				},
				Body: `{"superkey":"superval"}`,
			},
		},
		{
			"with_timestamp",
			func(p *Config) {
				parseFrom := entry.NewAttributeField("timestamp")
				p.TimeParser = &helper.TimeParser{
					ParseFrom:  &parseFrom,
					LayoutType: "epoch",
					Layout:     "s",
				}
			},
			&entry.Entry{
				Body: `{"superkey":"superval","timestamp":1136214245}`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"superkey":  "superval",
					"timestamp": float64(1136214245),
				},
				Body:      `{"superkey":"superval","timestamp":1136214245}`,
				Timestamp: time.Unix(1136214245, 0),
			},
		},
		{
			"with_scope",
			func(p *Config) {
				p.ScopeNameParser = &helper.ScopeNameParser{
					ParseFrom: entry.NewAttributeField("logger_name"),
				}
			},
			&entry.Entry{
				Body: `{"superkey":"superval","logger_name":"logger"}`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"superkey":    "superval",
					"logger_name": "logger",
				},
				Body:      `{"superkey":"superval","logger_name":"logger"}`,
				ScopeName: "logger",
			},
		},
		{
			"parse_ints_disabled",
			func(_ *Config) {},
			&entry.Entry{
				Body: `{"int":1,"float":1.0}`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"int":   float64(1),
					"float": float64(1),
				},
				Body: `{"int":1,"float":1.0}`,
			},
		},
		{
			"parse_ints_simple",
			func(p *Config) {
				p.ParseInts = true
			},
			&entry.Entry{
				Body: `{"int":1,"float":1.0}`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"int":   int64(1),
					"float": float64(1),
				},
				Body: `{"int":1,"float":1.0}`,
			},
		},
		{
			"parse_ints_nested",
			func(p *Config) {
				p.ParseInts = true
			},
			&entry.Entry{
				Body: `{"int":1,"float":1.0,"nested":{"int":2,"float":2.0}}`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"int":   int64(1),
					"float": float64(1),
					"nested": map[string]any{
						"int":   int64(2),
						"float": float64(2),
					},
				},
				Body: `{"int":1,"float":1.0,"nested":{"int":2,"float":2.0}}`,
			},
		},
		{
			"parse_ints_arrays",
			func(p *Config) {
				p.ParseInts = true
			},
			&entry.Entry{
				Body: `{"int":1,"float":1.0,"nested":{"int":2,"float":2.0},"array":[1,2]}`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"int":   int64(1),
					"float": float64(1),
					"nested": map[string]any{
						"int":   int64(2),
						"float": float64(2),
					},
					"array": []any{int64(1), int64(2)},
				},
				Body: `{"int":1,"float":1.0,"nested":{"int":2,"float":2.0},"array":[1,2]}`,
			},
		},
		{
			"parse_ints_mixed_arrays",
			func(p *Config) {
				p.ParseInts = true
			},
			&entry.Entry{
				Body: `{"int":1,"float":1.0,"mixed_array":[1,1.5,2]}`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"int":         int64(1),
					"float":       float64(1),
					"mixed_array": []any{int64(1), float64(1.5), int64(2)},
				},
				Body: `{"int":1,"float":1.0,"mixed_array":[1,1.5,2]}`,
			},
		},
		{
			"parse_ints_nested_arrays",
			func(p *Config) {
				p.ParseInts = true
			},
			&entry.Entry{
				Body: `{"int":1,"float":1.0,"nested":{"int":2,"float":2.0,"array":[1,2]},"array":[3,4]}`,
			},
			&entry.Entry{
				Attributes: map[string]any{
					"int":   int64(1),
					"float": float64(1),
					"nested": map[string]any{
						"int":   int64(2),
						"float": float64(2),
						"array": []any{int64(1), int64(2)},
					},
					"array": []any{int64(3), int64(4)},
				},
				Body: `{"int":1,"float":1.0,"nested":{"int":2,"float":2.0,"array":[1,2]},"array":[3,4]}`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfigWithID("test")
			cfg.OutputIDs = []string{"fake"}
			tc.configure(cfg)

			set := componenttest.NewNopTelemetrySettings()
			op, err := cfg.Build(set)
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

			ots := time.Now()
			tc.input.ObservedTimestamp = ots
			tc.expect.ObservedTimestamp = ots

			err = op.Process(t.Context(), tc.input)
			require.NoError(t, err)
			fake.ExpectEntry(t, tc.expect)
		})
	}
}

func BenchmarkProcess(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	cfg := NewConfig()

	parser, err := cfg.Build(componenttest.NewNopTelemetrySettings())
	require.NoError(b, err)

	benchmarkOperator(b, parser)
}

func BenchmarkProcessParseInts(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	cfg := NewConfig()
	cfg.ParseInts = true

	parser, err := cfg.Build(componenttest.NewNopTelemetrySettings())
	require.NoError(b, err)

	benchmarkOperator(b, parser)
}

func benchmarkOperator(b *testing.B, parser operator.Operator) {
	body, err := os.ReadFile(filepath.Join("testdata", "testdata.json"))
	require.NoError(b, err)

	e := entry.Entry{Body: string(body)}

	for b.Loop() {
		err := parser.Process(b.Context(), &e)
		require.NoError(b, err)
	}
}
