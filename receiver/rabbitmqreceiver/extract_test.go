// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestResolveExtractRules(t *testing.T) {
	rules, err := resolveExtractRules([]FieldExtractConfig{
		{TagName: "owner", Key: "owner"},
		{TagName: "$1", KeyRegex: "x-(.*)"},
	})
	require.NoError(t, err)
	require.Len(t, rules, 2)
	require.Equal(t, "owner", rules[0].tagName)
	require.Equal(t, "owner", rules[0].key)
	require.Nil(t, rules[0].keyRegex)
	require.NotNil(t, rules[1].keyRegex)

	_, err = resolveExtractRules([]FieldExtractConfig{{KeyRegex: "("}})
	require.Error(t, err)
}

func TestApplyArgumentExtraction(t *testing.T) {
	testCases := []struct {
		desc      string
		rules     []FieldExtractConfig
		existing  map[string]string
		arguments map[string]any
		expected  map[string]string
	}{
		{
			desc:      "exact key match",
			rules:     []FieldExtractConfig{{TagName: "owner", Key: "owner"}},
			arguments: map[string]any{"owner": "billing"},
			expected:  map[string]string{"owner": "billing"},
		},
		{
			desc:      "exact key match, no tag_name set uses argument's own key as attribute name",
			rules:     []FieldExtractConfig{{Key: "owner"}},
			arguments: map[string]any{"owner": "billing"},
			expected:  map[string]string{"owner": "billing"},
		},
		{
			desc:      "exact key not present is a no-op",
			rules:     []FieldExtractConfig{{TagName: "owner", Key: "owner"}},
			arguments: map[string]any{"other-key": "value"},
			expected:  map[string]string{},
		},
		{
			desc:      "key_regex without capture group uses argument's own key as attribute name",
			rules:     []FieldExtractConfig{{KeyRegex: "x-dead-letter-exchange"}},
			arguments: map[string]any{"x-dead-letter-exchange": "dlx"},
			expected:  map[string]string{"x-dead-letter-exchange": "dlx"},
		},
		{
			desc:      "key_regex with backreference tag_name",
			rules:     []FieldExtractConfig{{TagName: "$1", KeyRegex: "x-(.*)"}},
			arguments: map[string]any{"x-expires": "30000"},
			expected:  map[string]string{"expires": "30000"},
		},
		{
			desc:      "key_regex matches multiple arguments",
			rules:     []FieldExtractConfig{{TagName: "$1", KeyRegex: "x-(.*)"}},
			arguments: map[string]any{"x-expires": "30000", "x-max-length": "100", "unrelated": "ignored"},
			expected:  map[string]string{"expires": "30000", "max-length": "100"},
		},
		{
			desc:      "non-string argument values are stringified",
			rules:     []FieldExtractConfig{{TagName: "ttl", Key: "x-expires"}},
			arguments: map[string]any{"x-expires": float64(30000)},
			expected:  map[string]string{"ttl": "30000"},
		},
		{
			desc:      "large numeric argument values are not rendered in exponent form",
			rules:     []FieldExtractConfig{{TagName: "ttl", Key: "x-expires"}},
			arguments: map[string]any{"x-expires": float64(3600000)},
			expected:  map[string]string{"ttl": "3600000"},
		},
		{
			desc:      "nil arguments map is a no-op",
			rules:     []FieldExtractConfig{{TagName: "owner", Key: "owner"}},
			arguments: nil,
			expected:  map[string]string{},
		},
		{
			desc:      "extracted attribute does not overwrite an existing resource attribute",
			rules:     []FieldExtractConfig{{TagName: "rabbitmq.queue.name", Key: "owner"}},
			existing:  map[string]string{"rabbitmq.queue.name": "webq1"},
			arguments: map[string]any{"owner": "billing"},
			expected:  map[string]string{"rabbitmq.queue.name": "webq1"},
		},
		{
			desc:      "first rule to produce an attribute name wins",
			rules:     []FieldExtractConfig{{TagName: "owner", Key: "x-owner"}, {TagName: "owner", Key: "owner"}},
			arguments: map[string]any{"owner": "billing", "x-owner": "payments"},
			expected:  map[string]string{"owner": "payments"},
		},
		{
			desc:      "key_regex matching several keys onto one attribute name picks the lexically first key",
			rules:     []FieldExtractConfig{{TagName: "dlx", KeyRegex: "x-dead-letter-.*"}},
			arguments: map[string]any{"x-dead-letter-routing-key": "rk", "x-dead-letter-exchange": "dlx-exchange"},
			expected:  map[string]string{"dlx": "dlx-exchange"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rules, err := resolveExtractRules(tc.rules)
			require.NoError(t, err)

			resource := pcommon.NewResource()
			for k, v := range tc.existing {
				resource.Attributes().PutStr(k, v)
			}
			applyArgumentExtraction(rules, tc.arguments, resource)

			actual := map[string]string{}
			resource.Attributes().Range(func(k string, v pcommon.Value) bool {
				actual[k] = v.AsString()
				return true
			})
			require.Equal(t, tc.expected, actual)
		})
	}
}
