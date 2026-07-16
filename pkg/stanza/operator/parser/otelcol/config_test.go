// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcol

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestConfig(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:   "default",
				Expect: NewConfig(),
			},
			{
				Name: "on_error_drop",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.OnError = "drop"
					return cfg
				}(),
			},
			{
				Name: "format_json",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Format = formatJSON
					return cfg
				}(),
			},
			{
				Name: "format_console",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Format = formatConsole
					return cfg
				}(),
			},
			{
				Name: "format_auto",
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.Format = formatAuto
					return cfg
				}(),
			},
		},
	}.Run(t)
}

// TestConfigBuildIgnoresParseFromParseTo asserts that parse_from/parse_to
// are always forced to body/attributes by Build(), regardless of what a
// user sets in config. The unmarshal test above only checks that YAML
// unmarshals into the struct fields - it would NOT catch a regression
// where Build() stopped overriding them, or overrode them to the wrong
// value (e.g. writing parsed fields to body instead of attributes, which
// would silently discard them when postProcess overwrites e.Body).
func TestConfigBuildIgnoresParseFromParseTo(t *testing.T) {
	cfg := NewConfigWithID("test")

	// Deliberately set these to something other than the fixed values.
	cfg.ParseFrom = entry.NewBodyField("some_other_field")
	cfg.ParseTo = entry.RootableField{Field: entry.NewAttributeField("some_other_field")}

	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)

	parser, ok := op.(*Parser)
	require.True(t, ok)

	require.Equal(t, entry.NewBodyField(), parser.ParseFrom)
	require.Equal(t, entry.NewAttributeField(), parser.ParseTo)

	// End-to-end: confirm a real entry is still read from body and its
	// parsed fields still land in attributes, not "some_other_field".
	e := entry.New()
	e.Body = `{"ts":"2026-07-06T22:56:21.989Z","level":"info","msg":"hello","extra":"value"}`

	err = parser.Process(context.Background(), e)
	require.NoError(t, err)

	require.Equal(t, "hello", e.Body)
	require.Equal(t, "value", e.Attributes["extra"])
}