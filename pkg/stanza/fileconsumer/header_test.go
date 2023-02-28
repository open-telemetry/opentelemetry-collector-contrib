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
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/generate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"
)

func TestHeaderConfig_validate(t *testing.T) {
	regexConf := regex.NewConfig()
	regexConf.Regex = "^#(?P<header_line>.*)"

	invalidRegexConf := regex.NewConfig()
	invalidRegexConf.Regex = "("

	generateConf := generate.NewConfig("")

	defaultMaxHeaderByteSize := helper.ByteSize(defaultMaxHeaderSize)
	negativeMaxHeaderByteSize := helper.ByteSize(-1)

	testCases := []struct {
		name        string
		conf        HeaderConfig
		expectedErr string
	}{
		{
			name: "Valid config",
			conf: HeaderConfig{
				LineStartPattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: regexConf,
					},
				},
				MaxHeaderLineSize: &defaultMaxHeaderByteSize,
			},
		},
		{
			name: "Valid without specified header size",
			conf: HeaderConfig{
				LineStartPattern: "^#",
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
				LineStartPattern: "(",
				MetadataOperators: []operator.Config{
					{
						Builder: regexConf,
					},
				},
				MaxHeaderLineSize: &defaultMaxHeaderByteSize,
			},
			expectedErr: "invalid `multiline_pattern`:",
		},
		{
			name: "Negative max header size",
			conf: HeaderConfig{
				LineStartPattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: regexConf,
					},
				},
				MaxHeaderLineSize: &negativeMaxHeaderByteSize,
			},
			expectedErr: "the `max_size` of the header must be greater than 0",
		},
		{
			name: "No operators specified",
			conf: HeaderConfig{
				LineStartPattern:  "^#",
				MetadataOperators: []operator.Config{},
				MaxHeaderLineSize: &defaultMaxHeaderByteSize,
			},
			expectedErr: "at least one operator must be specified for `metadata_operators`",
		},
		{
			name: "Invalid operator specified",
			conf: HeaderConfig{
				LineStartPattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: invalidRegexConf,
					},
				},
				MaxHeaderLineSize: &defaultMaxHeaderByteSize,
			},
			expectedErr: "failed to build pipelines:",
		},
		{
			name: "first operator cannot process",
			conf: HeaderConfig{
				LineStartPattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: generateConf,
					},
				},
				MaxHeaderLineSize: &defaultMaxHeaderByteSize,
			},
			expectedErr: "first operator must be able to process entries",
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

func TestHeaderConfig_buildHeader(t *testing.T) {
	regexConf := regex.NewConfig()
	regexConf.Regex = "^#(?P<header_line>.*)"

	invalidRegexConf := regex.NewConfig()
	invalidRegexConf.Regex = "("

	badEncoding := mockEncoding{
		encodingTransformer: errorTransformer{
			e: errors.New("cannot encode bytes"),
		},
	}

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
				LineStartPattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: regexConf,
					},
				},
			},
		},
		{
			name: "invalid regex",
			enc:  encoding.Nop,
			conf: HeaderConfig{
				LineStartPattern: "^(",
				MetadataOperators: []operator.Config{
					{
						Builder: regexConf,
					},
				},
			},
			expectedErr: "failed to compile multiline pattern:",
		},
		{
			name: "invalid operator",
			enc:  encoding.Nop,
			conf: HeaderConfig{
				LineStartPattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: invalidRegexConf,
					},
				},
			},
			expectedErr: "failed to build pipeline:",
		},
		{
			name: "invalid encoding",
			enc:  badEncoding,
			conf: HeaderConfig{
				LineStartPattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: regexConf,
					},
				},
			},
			expectedErr: "failed to create split func",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := tc.conf.buildHeader(zaptest.NewLogger(t).Sugar(), tc.enc, nil)
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

	smallByteSize := helper.ByteSize(8)

	testCases := []struct {
		name               string
		fileContents       string
		expectedAttributes map[string]any
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
				LineStartPattern: "^#",
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
				LineStartPattern: "^#",
				MetadataOperators: []operator.Config{
					{
						Builder: fullCaptureRegexConfig,
					},
				},
				MaxHeaderLineSize: &smallByteSize,
			},
		},
		{
			name:         "Header attribute from following line overwrites previous",
			fileContents: "#aField: SomeValue\n#secondValue: SomeValue2\nThis is a non-header line\n",
			expectedAttributes: map[string]any{
				"header": "#secondValue: SomeValue2",
			},
			conf: HeaderConfig{
				LineStartPattern: "^#",
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
				LineStartPattern: "^#",
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
				LineStartPattern: "^#",
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

			h, err := tc.conf.buildHeader(zaptest.NewLogger(t).Sugar(), enc.Encoding, nil)
			require.NoError(t, err)

			r := bytes.NewReader([]byte(tc.fileContents))

			fa := &FileAttributes{}

			h.ReadHeader(context.Background(), r, enc, fa)

			require.Equal(t, tc.expectedAttributes, fa.HeaderAttributes)
			require.NoError(t, h.Shutdown())
		})
	}
}

type mockEncoding struct {
	encodingTransformer transform.Transformer
}

func (m mockEncoding) NewEncoder() *encoding.Encoder {
	return &encoding.Encoder{
		Transformer: m.encodingTransformer,
	}
}

func (m mockEncoding) NewDecoder() *encoding.Decoder {
	// Unimplemented
	return nil
}

// errorTransformer is a mock transform.Transformer that always returns the provided error
type errorTransformer struct {
	e error
}

func (et errorTransformer) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	return 0, 0, et.e
}

func (errorTransformer) Reset() {}
