// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package json

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func newTestParser(t *testing.T) *Parser {
	config := NewConfigWithID("test")
	op, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
	return op.(*Parser)
}

func TestConfigBuild(t *testing.T) {
	config := NewConfigWithID("test")
	op, err := config.Build(testutil.Logger(t))
	require.NoError(t, err)
	require.IsType(t, &Parser{}, op)
}

func TestConfigBuildFailure(t *testing.T) {
	config := NewConfigWithID("test")
	config.OnError = "invalid_on_error"
	_, err := config.Build(testutil.Logger(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid `on_error` field")
}

func TestParserStringFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("invalid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "error found in #1 byte")
}

func TestParserByteFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]byte("invalid"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "type []uint8 cannot be parsed as JSON")
}

func TestParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "type []int cannot be parsed as JSON")
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
			func(p *Config) {},
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
			func(p *Config) {},
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewConfigWithID("test")
			cfg.OutputIDs = []string{"fake"}
			tc.configure(cfg)

			op, err := cfg.Build(testutil.Logger(t))
			require.NoError(t, err)

			fake := testutil.NewFakeOutput(t)
			require.NoError(t, op.SetOutputs([]operator.Operator{fake}))

			ots := time.Now()
			tc.input.ObservedTimestamp = ots
			tc.expect.ObservedTimestamp = ots

			err = op.Process(context.Background(), tc.input)
			require.NoError(t, err)
			fake.ExpectEntry(t, tc.expect)
		})
	}
}
