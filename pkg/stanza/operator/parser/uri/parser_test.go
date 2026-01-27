// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package uri

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

func newTestParser(t *testing.T) *Parser {
	cfg := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	return op.(*Parser)
}

func TestInit(t *testing.T) {
	builder, ok := operator.DefaultRegistry.Lookup("uri_parser")
	require.True(t, ok, "expected uri_parser to be registered")
	require.Equal(t, "uri_parser", builder().Type())
}

func TestParserBuildFailure(t *testing.T) {
	cfg := NewConfigWithID("test")
	cfg.OnError = "invalid_on_error"
	set := componenttest.NewNopTelemetrySettings()
	_, err := cfg.Build(set)
	require.ErrorContains(t, err, "invalid `on_error` field")
}

func TestParserByteFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]byte("invalid"))
	require.ErrorContains(t, err, "type '[]uint8' cannot be parsed as URI")
}

func TestParserStringFailure(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("invalid")
	require.ErrorContains(t, err, "parse \"invalid\": invalid URI for request")
}

func TestParserInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse([]int{})
	require.ErrorContains(t, err, "type '[]int' cannot be parsed as URI")
}

func TestProcess(t *testing.T) {
	cases := []struct {
		name   string
		op     func() (operator.Operator, error)
		input  *entry.Entry
		expect *entry.Entry
	}{
		{
			"default",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			&entry.Entry{
				Body: "https://google.com:443/path?user=dev",
			},
			&entry.Entry{
				Attributes: map[string]any{
					"host": "google.com",
					"port": "443",
					"path": "/path",
					"query": map[string]any{
						"user": []any{
							"dev",
						},
					},
					"scheme": "https",
				},
				Body: "https://google.com:443/path?user=dev",
			},
		},
		{
			"parse-to",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.ParseFrom = entry.NewBodyField("url")
				cfg.ParseTo = entry.RootableField{Field: entry.NewBodyField("url2")}
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			&entry.Entry{
				Body: map[string]any{
					"url": "https://google.com:443/path?user=dev",
				},
			},
			&entry.Entry{
				Body: map[string]any{
					"url": "https://google.com:443/path?user=dev",
					"url2": map[string]any{
						"host": "google.com",
						"port": "443",
						"path": "/path",
						"query": map[string]any{
							"user": []any{
								"dev",
							},
						},
						"scheme": "https",
					},
				},
			},
		},
		{
			"parse-from",
			func() (operator.Operator, error) {
				cfg := NewConfigWithID("test_id")
				cfg.ParseFrom = entry.NewBodyField("url")
				set := componenttest.NewNopTelemetrySettings()
				return cfg.Build(set)
			},
			&entry.Entry{
				Body: map[string]any{
					"url": "https://google.com:443/path?user=dev",
				},
			},
			&entry.Entry{
				Body: map[string]any{
					"url": "https://google.com:443/path?user=dev",
				},
				Attributes: map[string]any{
					"host": "google.com",
					"port": "443",
					"path": "/path",
					"query": map[string]any{
						"user": []any{
							"dev",
						},
					},
					"scheme": "https",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := tc.op()
			require.NoError(t, err, "did not expect operator function to return an error, this is a bug with the test case")

			err = op.Process(t.Context(), tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expect, tc.input)
		})
	}
}

func TestParserParse(t *testing.T) {
	cases := []struct {
		name       string
		inputBody  any
		outputBody map[string]any
		expectErr  bool
	}{
		{
			"string",
			"http://www.google.com/app?env=prod",
			map[string]any{
				"scheme": "http",
				"host":   "www.google.com",
				"path":   "/app",
				"query": map[string]any{
					"env": []any{
						"prod",
					},
				},
			},
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := Parser{}
			x, err := parser.parse(tc.inputBody)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.outputBody, x)
		})
	}
}

func TestBuildParserURL(t *testing.T) {
	newBasicParser := func() *Config {
		cfg := NewConfigWithID("test")
		cfg.OutputIDs = []string{"test"}
		return cfg
	}

	t.Run("BasicConfig", func(t *testing.T) {
		c := newBasicParser()
		set := componenttest.NewNopTelemetrySettings()
		_, err := c.Build(set)
		require.NoError(t, err)
	})
}

func BenchmarkParserParse(b *testing.B) {
	v := "https://dev:password@www.golang.org:8443/v1/app/stage?token=d9e28b1d-2c7b-4853-be6a-d94f34a5d4ab&env=prod&env=stage&token=c6fa29f9-a31b-4584-b98d-aa8473b0e18d&region=us-east1b&mode=fast"
	parser := Parser{}
	for b.Loop() {
		if _, err := parser.parse(v); err != nil {
			b.Fatal(err)
		}
	}
}
