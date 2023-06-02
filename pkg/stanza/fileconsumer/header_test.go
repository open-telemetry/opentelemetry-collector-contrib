// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/generate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/stdout"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/filter"
)

func TestHeaderConfig_validate(t *testing.T) {
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
		conf        HeaderConfig
		expectedErr string
	}{
		{
			name: "Valid config",
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: regexConf,
					},
				},
			},
		},
		{
			name: "Valid without specified header size",
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: regexConf,
					},
				},
			},
		},
		{
			name: "No operators specified",
			conf: HeaderConfig{
				Pattern:           "^#",
				MetadataOperators: []operator.Config{},
			},
			expectedErr: "at least one operator must be specified for `metadata_operators`",
		},
		{
			name: "Invalid operator specified",
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: invalidRegexConf,
					},
				},
			},
			expectedErr: "failed to build pipelines:",
		},
		{
			name: "first operator cannot process",
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: generateConf,
					},
				},
			},
			expectedErr: "operator 'generate_input' in `metadata_operators` cannot process entries",
		},
		{
			name: "operator cannot output",
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: stdoutConf,
					},
				},
			},
			expectedErr: "operator 'stdout' in `metadata_operators` does not propagate entries",
		},
		{
			name: "filter operator present",
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: filterConfg,
					},
				},
			},
			expectedErr: "operator of type filter is not allowed in `metadata_operators`",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.conf.validate()
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHeaderConfig_buildHeaderSettings(t *testing.T) {
	regexConf := regex.NewConfig()
	regexConf.Regex = "^#(?P<header_line>.*)"

	invalidRegexConf := regex.NewConfig()
	invalidRegexConf.Regex = "("

	testCases := []struct {
		name        string
		enc         encoding.Encoding
		conf        HeaderConfig
		expectedErr string
	}{
		{
			name: "valid config",
			enc:  encoding.Nop,
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: regexConf,
					},
				},
			},
		},
		{
			name: "Invalid pattern",
			conf: HeaderConfig{
				Pattern: "(",
				MetadataOperators: []operator.Config{
					{
						Builder: regexConf,
					},
				},
			},
			expectedErr: "failed to compile `pattern`:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := tc.conf.buildHeaderSettings(tc.enc)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, h)
			}

		})
	}
}
