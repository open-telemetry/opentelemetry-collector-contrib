// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package header

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/generate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/stdout"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/filter"
)

func TestBuild(t *testing.T) {
	regexConf := regex.NewConfig()
	regexConf.Regex = "^#(?P<header_line>.*)"

	invalidRegexConf := regex.NewConfig()
	invalidRegexConf.Regex = "("

	generateConf := generate.NewConfig("")
	stdoutConf := stdout.NewConfig("")
	filterConfg := filter.NewConfig()
	filterConfg.Expression = "true"

	testCases := []struct {
		name        string
		enc         encoding.Encoding
		pattern     string
		ops         []operator.Config
		expectedErr string
	}{
		{
			name:    "valid config",
			enc:     encoding.Nop,
			pattern: "^#",
			ops: []operator.Config{
				{
					Builder: regexConf,
				},
			},
		},
		{
			name:    "Invalid pattern",
			enc:     unicode.UTF8,
			pattern: "(",
			ops: []operator.Config{
				{
					Builder: regexConf,
				},
			},
			expectedErr: "failed to compile `pattern`:",
		},
		{
			name:    "Valid without specified header size",
			enc:     unicode.UTF8,
			pattern: "^#",
			ops: []operator.Config{
				{
					Builder: regexConf,
				},
			},
		},
		{
			name:        "No operators specified",
			enc:         unicode.UTF8,
			pattern:     "^#",
			ops:         []operator.Config{},
			expectedErr: "at least one operator must be specified for `metadata_operators`",
		},
		{
			name:    "No encoding specified",
			pattern: "^#",
			ops: []operator.Config{
				{
					Builder: regexConf,
				},
			},
			expectedErr: "encoding must be specified",
		},
		{
			name:    "Invalid operator specified",
			enc:     unicode.UTF8,
			pattern: "^#",
			ops: []operator.Config{
				{
					Builder: invalidRegexConf,
				},
			},
			expectedErr: "failed to build pipelines:",
		},
		{
			name:    "first operator cannot process",
			enc:     unicode.UTF8,
			pattern: "^#",
			ops: []operator.Config{
				{
					Builder: generateConf,
				},
			},
			expectedErr: "operator 'generate_input' in `metadata_operators` cannot process entries",
		},
		{
			name:    "operator cannot output",
			enc:     unicode.UTF8,
			pattern: "^#",
			ops: []operator.Config{
				{
					Builder: stdoutConf,
				},
			},
			expectedErr: "operator 'stdout' in `metadata_operators` does not propagate entries",
		},
		{
			name:    "filter operator present",
			enc:     unicode.UTF8,
			pattern: "^#",
			ops: []operator.Config{
				{
					Builder: filterConfg,
				},
			},
			expectedErr: "operator of type filter is not allowed in `metadata_operators`",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := NewConfig(tc.pattern, tc.ops, tc.enc)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, h)
			}
		})
	}
}
