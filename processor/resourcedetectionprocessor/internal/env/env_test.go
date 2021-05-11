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

package env

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(component.ProcessorCreateParams{Logger: zap.NewNop()}, nil)
	assert.NotNil(t, d)
	assert.NoError(t, err)
}

func TestDetectTrue(t *testing.T) {
	os.Setenv(envVar, "key=value")

	detector := &Detector{}
	res, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, internal.NewResource(map[string]interface{}{"key": "value"}), res)
}

func TestDetectFalse(t *testing.T) {
	os.Setenv(envVar, "")

	detector := &Detector{}
	res, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
}

func TestDetectDeprecatedEnv(t *testing.T) {
	os.Setenv(envVar, "")
	os.Setenv(deprecatedEnvVar, "key=value")

	detector := &Detector{}
	res, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, internal.NewResource(map[string]interface{}{"key": "value"}), res)
}

func TestDetectError(t *testing.T) {
	os.Setenv(envVar, "key=value,key")

	detector := &Detector{}
	res, err := detector.Detect(context.Background())
	assert.Error(t, err)
	assert.True(t, internal.IsEmptyResource(res))
}

func TestInitializeAttributeMap(t *testing.T) {
	cases := []struct {
		name               string
		encoded            string
		expectedAttributes pdata.AttributeMap
		expectedError      string
	}{
		{
			name:               "multiple valid attributes",
			encoded:            ` example.org/test-1 =  test $ %3A \" ,  Abc=Def  `,
			expectedAttributes: internal.NewAttributeMap(map[string]interface{}{"example.org/test-1": `test $ : \"`, "Abc": "Def"}),
		}, {
			name:               "single valid attribute",
			encoded:            `single=key`,
			expectedAttributes: internal.NewAttributeMap(map[string]interface{}{"single": "key"}),
		}, {
			name:          "invalid url escape sequence in value",
			encoded:       `invalid=url-%3-encoding`,
			expectedError: `invalid resource format in attribute: "invalid=url-%3-encoding", err: invalid URL escape "%3-"`,
		}, {
			name:          "invalid char in key",
			encoded:       `invalid-char-ü=test`,
			expectedError: `invalid resource format: "invalid-char-ü=test"`,
		}, {
			name:          "invalid char in value",
			encoded:       `invalid-char=ü-test`,
			expectedError: `invalid resource format: "invalid-char=ü-test"`,
		}, {
			name:          "invalid attribute",
			encoded:       `extra=chars, a`,
			expectedError: `invalid resource format, invalid text: " a"`,
		}, {
			name:          "invalid char between attributes",
			encoded:       `invalid=char,übetween=attributes`,
			expectedError: `invalid resource format, invalid text: "ü"`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			am := pdata.NewAttributeMap()
			err := initializeAttributeMap(am, c.encoded)

			if c.expectedError != "" {
				assert.EqualError(t, err, c.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, c.expectedAttributes.Sort(), am.Sort())
			}
		})
	}
}
