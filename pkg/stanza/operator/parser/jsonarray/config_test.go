// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package jsonarray

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	commontestutil "github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/operatortest"
)

func TestConfig(t *testing.T) {
	// Manually adding operator to the Registry as it's behind a feature gate
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })

	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name: "basic",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseFrom = entry.NewBodyField("message")
					p.ParseTo = entry.RootableField{Field: entry.NewBodyField("messageParsed")}
					return p
				}(),
			},
			{
				Name: "parse_to_attributes",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseFrom = entry.NewBodyField()
					p.ParseTo = entry.RootableField{Field: entry.NewAttributeField("output")}
					return p
				}(),
			},
			{
				Name: "parse_to_body",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = entry.RootableField{Field: entry.NewBodyField()}
					return p
				}(),
			},
			{
				Name: "parse_to_resource",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = entry.RootableField{Field: entry.NewResourceField("output")}
					return p
				}(),
			},
			{
				Name: "parse_with_header_as_attributes",
				Expect: func() *Config {
					p := NewConfig()
					p.ParseTo = entry.RootableField{Field: entry.NewAttributeField()}
					p.Header = "A,B,C"
					return p
				}(),
			},
		},
	}.Run(t)
}

func TestBuildWithFeatureGate(t *testing.T) {
	cases := []struct {
		name                string
		isFeatureGateEnable bool
		onErr               string
	}{
		{"jsonarray_enabled", true, ""},
		{"jsonarray_disabled", false, "operator disabled"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.isFeatureGateEnable {
				defer commontestutil.SetFeatureGateForTest(t, jsonArrayParserFeatureGate, true)()
			}

			buildFunc, ok := operator.Lookup(operatorType)
			require.True(t, ok)

			set := componenttest.NewNopTelemetrySettings()
			_, err := buildFunc().Build(set)
			if err != nil {
				require.ErrorContains(t, err, c.onErr)
			}
		})
	}
}
