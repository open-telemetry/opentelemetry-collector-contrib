// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestIdentifier(t *testing.T) {
	t.Setenv("TEST_METADATA_OPERATOR_ENV", "foo")

	cases := []struct {
		name     string
		config   IdentifierConfig
		input    *entry.Entry
		expected *entry.Entry
	}{
		{
			"AddAttributeLiteral",
			func() IdentifierConfig {
				cfg := NewIdentifierConfig()
				cfg.Resource = map[string]ExprStringConfig{
					"key1": "value1",
				}
				return cfg
			}(),
			entry.New(),
			func() *entry.Entry {
				e := entry.New()
				e.Resource = map[string]interface{}{
					"key1": "value1",
				}
				return e
			}(),
		},
		{
			"AddAttributeExpr",
			func() IdentifierConfig {
				cfg := NewIdentifierConfig()
				cfg.Resource = map[string]ExprStringConfig{
					"key1": `EXPR("start" + "end")`,
				}
				return cfg
			}(),
			entry.New(),
			func() *entry.Entry {
				e := entry.New()
				e.Resource = map[string]interface{}{
					"key1": "startend",
				}
				return e
			}(),
		},
		{
			"AddAttributeEnv",
			func() IdentifierConfig {
				cfg := NewIdentifierConfig()
				cfg.Resource = map[string]ExprStringConfig{
					"key1": `EXPR(env("TEST_METADATA_OPERATOR_ENV"))`,
				}
				return cfg
			}(),
			entry.New(),
			func() *entry.Entry {
				e := entry.New()
				e.Resource = map[string]interface{}{
					"key1": "foo",
				}
				return e
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			identifier, err := tc.config.Build()
			require.NoError(t, err)

			err = identifier.Identify(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expected.Resource, tc.input.Resource)
		})
	}
}
