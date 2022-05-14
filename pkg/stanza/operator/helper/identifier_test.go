// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestIdentifier(t *testing.T) {
	os.Setenv("TEST_METADATA_OPERATOR_ENV", "foo")
	defer os.Unsetenv("TEST_METADATA_OPERATOR_ENV")

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
