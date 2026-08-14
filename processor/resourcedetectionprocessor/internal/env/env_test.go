// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), nil, false)
	assert.NotNil(t, d)
	assert.NoError(t, err)
}

func TestDetectTrue(t *testing.T) {
	t.Setenv(envVar, "key=value")

	detector := &Detector{}
	res, schemaURL, err := detector.Detect(t.Context())
	assert.Empty(t, schemaURL)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"key": "value"}, res.Attributes().AsRaw())
}

func TestDetectFalse(t *testing.T) {
	t.Setenv(envVar, "")

	detector := &Detector{}
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Empty(t, schemaURL)
	assert.True(t, internal.IsEmptyResource(res))
}

func TestDetectDeprecatedEnv(t *testing.T) {
	t.Setenv(envVar, "")
	t.Setenv(deprecatedEnvVar, "key=value")

	detector := &Detector{}
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Empty(t, schemaURL)
	assert.Equal(t, map[string]any{"key": "value"}, res.Attributes().AsRaw())
}

func TestDetectIncluded(t *testing.T) {
	t.Setenv(envVar, "keep=1,drop=2,also_keep=3")

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), Config{
		Attributes: AttributesConfig{Included: []string{"keep", "also_keep"}},
	}, false)
	require.NoError(t, err)
	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"keep": "1", "also_keep": "3"}, res.Attributes().AsRaw())
}

func TestDetectIncludedWildcard(t *testing.T) {
	t.Setenv(envVar, "k8s.cluster.name=c,k8s.cluster.uid=u,k8s.pod.name=p,other=1")

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), Config{
		Attributes: AttributesConfig{
			Included: []string{"k8s.cluster.*"},
		},
	}, false)
	require.NoError(t, err)
	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k8s.cluster.name": "c", "k8s.cluster.uid": "u"}, res.Attributes().AsRaw())
}

func TestDetectExcludedAppliesAfterIncluded(t *testing.T) {
	t.Setenv(envVar, "k8s.cluster.name=c,k8s.pod.name=p,k8s.namespace.name=n,other=1")

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), Config{
		Attributes: AttributesConfig{
			Included: []string{"k8s.*"},
			Excluded: []string{"k8s.pod.name"},
		},
	}, false)
	require.NoError(t, err)
	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"k8s.cluster.name": "c", "k8s.namespace.name": "n"}, res.Attributes().AsRaw())
}

func TestDetectExcludedOnly(t *testing.T) {
	t.Setenv(envVar, "a=1,b=2,c=3")

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), Config{
		Attributes: AttributesConfig{Excluded: []string{"b"}},
	}, false)
	require.NoError(t, err)
	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"a": "1", "c": "3"}, res.Attributes().AsRaw())
}

func TestDetectDefaultConfigAllowsAll(t *testing.T) {
	t.Setenv(envVar, "a=1,b=2")

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"a": "1", "b": "2"}, res.Attributes().AsRaw())
}

func TestDetectError(t *testing.T) {
	t.Setenv(envVar, "key=value,key")

	detector := &Detector{}
	res, schemaURL, err := detector.Detect(t.Context())
	assert.Error(t, err)
	assert.Empty(t, schemaURL)
	assert.True(t, internal.IsEmptyResource(res))
}

func TestCompilePatterns(t *testing.T) {
	cases := []struct {
		name     string
		patterns []string
		matches  map[string]bool
	}{
		{
			name:     "nil returns nil",
			patterns: nil,
			matches:  nil,
		},
		{
			name:     "empty slice returns nil",
			patterns: []string{},
			matches:  nil,
		},
		{
			name:     "exact match",
			patterns: []string{"k8s.cluster.name"},
			matches: map[string]bool{
				"k8s.cluster.name":  true,
				"k8s.cluster.names": false,
				"k8s.cluster":       false,
				"":                  false,
			},
		},
		{
			name:     "trailing star wildcard",
			patterns: []string{"k8s.cluster.*"},
			matches: map[string]bool{
				"k8s.cluster.":     true,
				"k8s.cluster.name": true,
				"k8s.cluster.uid":  true,
				"k8s.cluster":      false,
				"k8s.pod.name":     false,
			},
		},
		{
			name:     "leading and middle star wildcard",
			patterns: []string{"*.name", "k8s.*.uid"},
			matches: map[string]bool{
				"host.name":       true,
				"k8s.cluster.uid": true,
				"k8s.pod.uid":     true,
				"other":           false,
			},
		},
		{
			name:     "regex metacharacters are escaped",
			patterns: []string{"a.b"},
			matches: map[string]bool{
				"a.b": true,
				"aXb": false,
			},
		},
		{
			name:     "multiple patterns any-match",
			patterns: []string{"foo", "bar.*"},
			matches: map[string]bool{
				"foo":     true,
				"bar.baz": true,
				"baz":     false,
			},
		},
		{
			name:     "case-sensitive",
			patterns: []string{"Foo"},
			matches: map[string]bool{
				"Foo": true,
				"foo": false,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			compiled, err := compilePatterns(c.patterns)
			require.NoError(t, err)
			if c.matches == nil {
				assert.Nil(t, compiled)
				return
			}
			assert.Len(t, compiled, len(c.patterns))
			for key, want := range c.matches {
				assert.Equalf(t, want, matchAny(compiled, key), "match(%q)", key)
			}
		})
	}
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
