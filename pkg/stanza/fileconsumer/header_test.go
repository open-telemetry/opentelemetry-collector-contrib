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

package fileconsumer

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
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

func TestHeaderConfig_build(t *testing.T) {
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
			err := tc.conf.build(tc.enc)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

		})
	}
}

func TestHeaderConfig_buildHeader(t *testing.T) {
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
		{
			name: "Invalid operator",
			enc:  encoding.Nop,
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: invalidRegexConf,
					},
				},
			},
			expectedErr: "failed to build pipeline:",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.conf.build(tc.enc))
			h, err := tc.conf.buildHeader(zaptest.NewLogger(t).Sugar())
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, h)
			}

		})
	}
}

func TestHeaderConfig_ReadHeader(t *testing.T) {
	basicRegexConfig := regex.NewConfig()
	basicRegexConfig.Regex = "^#(?P<field_name>[A-z0-9]*): (?P<value>[A-z0-9]*)"

	fullCaptureRegexConfig := regex.NewConfig()
	fullCaptureRegexConfig.Regex = `^(?P<header>[\s\S]*)$`

	captureFieldOneRegexConfig := regex.NewConfig()
	captureFieldOneRegexConfig.Regex = `^#aField: (?P<field1>.*)$`
	captureFieldOneRegexConfig.IfExpr = `body startsWith "#aField:"`

	captureFieldTwoRegexConfig := regex.NewConfig()
	captureFieldTwoRegexConfig.Regex = `^#secondValue: (?P<field2>.*)$`
	captureFieldTwoRegexConfig.IfExpr = `body startsWith "#secondValue:"`

	generateConf := generate.NewConfig("")

	testCases := []struct {
		name               string
		fileContents       string
		expectedAttributes map[string]any
		maxLineSize        int
		conf               HeaderConfig
	}{
		{
			name:         "Header + log line",
			fileContents: "#aField: SomeValue\nThis is a non-header line\n",
			expectedAttributes: map[string]any{
				"field_name": "aField",
				"value":      "SomeValue",
			},
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: basicRegexConfig,
					},
				},
			},
		},
		{
			name:         "Header truncates when too long",
			fileContents: "#aField: SomeValue\nThis is a non-header line\n",
			expectedAttributes: map[string]any{
				"header": "#aField:",
			},
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: fullCaptureRegexConfig,
					},
				},
			},
			maxLineSize: 8,
		},
		{
			name:         "Header attribute from following line overwrites previous",
			fileContents: "#aField: SomeValue\n#secondValue: SomeValue2\nThis is a non-header line\n",
			expectedAttributes: map[string]any{
				"header": "#secondValue: SomeValue2",
			},
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: fullCaptureRegexConfig,
					},
				},
			},
		},
		{
			name:         "Header attribute from both lines merged",
			fileContents: "#aField: SomeValue\n#secondValue: SomeValue2\nThis is a non-header line\n",
			expectedAttributes: map[string]any{
				"field1": "SomeValue",
				"field2": "SomeValue2",
			},
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: captureFieldOneRegexConfig,
					},
					{
						Builder: captureFieldTwoRegexConfig,
					},
				},
			},
		},
		{
			name:               "Pipeline starts with non-parser",
			fileContents:       "#aField: SomeValue\nThis is a non-header line\n",
			expectedAttributes: map[string]any{},
			conf: HeaderConfig{
				Pattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: generateConf,
					},
					{
						Builder: basicRegexConfig,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encConf := helper.NewEncodingConfig()
			encConf.Encoding = "utf8"
			enc, err := encConf.Build()
			require.NoError(t, err)

			require.NoError(t, tc.conf.build(enc.Encoding))

			h, err := tc.conf.buildHeader(zaptest.NewLogger(t).Sugar())
			require.NoError(t, err)

			r := bytes.NewReader([]byte(tc.fileContents))

			fa := &FileAttributes{}

			var maxLineSize int
			if tc.maxLineSize == 0 {
				maxLineSize = defaultMaxLogSize
			} else {
				maxLineSize = tc.maxLineSize
			}

			h.ReadHeader(context.Background(), r, maxLineSize, enc, fa)

			require.Equal(t, tc.expectedAttributes, fa.HeaderAttributes)
			require.NoError(t, h.Shutdown())
		})
	}
}
