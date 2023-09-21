// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(processortest.NewNopCreateSettings(), nil)
	assert.NotNil(t, d)
	assert.NoError(t, err)
}

func TestDetectTrue(t *testing.T) {
	t.Setenv(envVar, "key=value")

	detector := &Detector{}
	res, schemaURL, err := detector.Detect(context.Background())
	assert.Equal(t, "", schemaURL)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"key": "value"}, res.Attributes().AsRaw())
}

func TestDetectFalse(t *testing.T) {
	t.Setenv(envVar, "")

	detector := &Detector{}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "", schemaURL)
	assert.True(t, internal.IsEmptyResource(res))
}

func TestDetectDeprecatedEnv(t *testing.T) {
	t.Setenv(envVar, "")
	t.Setenv(deprecatedEnvVar, "key=value")

	detector := &Detector{}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "", schemaURL)
	assert.Equal(t, map[string]any{"key": "value"}, res.Attributes().AsRaw())
}

func TestDetectError(t *testing.T) {
	t.Setenv(envVar, "key=value,key")

	detector := &Detector{}
	res, schemaURL, err := detector.Detect(context.Background())
	assert.Error(t, err)
	assert.Equal(t, "", schemaURL)
	assert.True(t, internal.IsEmptyResource(res))
}

func TestInitializeAttributeMap(t *testing.T) {
	cases := []struct {
		name               string
		encoded            string
		expectedAttributes map[string]any
		expectedError      string
	}{
		{
			name:               "multiple valid attributes",
			encoded:            ` example.org/test-1 =  test $ %3A \" ,  Abc=Def  `,
			expectedAttributes: map[string]any{"example.org/test-1": `test $ : \"`, "Abc": "Def"},
		}, {
			name:               "single valid attribute",
			encoded:            `single=key`,
			expectedAttributes: map[string]any{"single": "key"},
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
			am := pcommon.NewMap()
			err := initializeAttributeMap(am, c.encoded)

			if c.expectedError != "" {
				assert.EqualError(t, err, c.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, c.expectedAttributes, am.AsRaw())
			}
		})
	}
}
