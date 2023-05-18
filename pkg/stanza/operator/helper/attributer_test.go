// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestAttributer(t *testing.T) {
	t.Setenv("TEST_METADATA_OPERATOR_ENV", "foo")

	cases := []struct {
		name     string
		config   AttributerConfig
		input    *entry.Entry
		expected *entry.Entry
	}{
		{
			"AddAttributeLiteral",
			func() AttributerConfig {
				cfg := NewAttributerConfig()
				cfg.Attributes = map[string]ExprStringConfig{
					"label1": "value1",
				}
				return cfg
			}(),
			entry.New(),
			func() *entry.Entry {
				e := entry.New()
				e.Attributes = map[string]interface{}{
					"label1": "value1",
				}
				return e
			}(),
		},
		{
			"AddAttributeExpr",
			func() AttributerConfig {
				cfg := NewAttributerConfig()
				cfg.Attributes = map[string]ExprStringConfig{
					"label1": `EXPR("start" + "end")`,
				}
				return cfg
			}(),
			entry.New(),
			func() *entry.Entry {
				e := entry.New()
				e.Attributes = map[string]interface{}{
					"label1": "startend",
				}
				return e
			}(),
		},
		{
			"AddAttributeEnv",
			func() AttributerConfig {
				cfg := NewAttributerConfig()
				cfg.Attributes = map[string]ExprStringConfig{
					"label1": `EXPR(env("TEST_METADATA_OPERATOR_ENV"))`,
				}
				return cfg
			}(),
			entry.New(),
			func() *entry.Entry {
				e := entry.New()
				e.Attributes = map[string]interface{}{
					"label1": "foo",
				}
				return e
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attributer, err := tc.config.Build()
			require.NoError(t, err)

			err = attributer.Attribute(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expected.Attributes, tc.input.Attributes)
		})
	}
}
