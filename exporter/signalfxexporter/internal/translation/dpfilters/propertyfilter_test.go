// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dpfilters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"gopkg.in/yaml.v3"
)

func TestPropertyFilterUnmarshaling(t *testing.T) {
	for _, test := range []struct {
		name                   string
		yaml                   string
		expectedPropertyFilter PropertyFilter
		expectedError          string
	}{
		{
			name: "happy path",
			yaml: `dimension_name: some.dimension.name
dimension_value: some.dimension.value
property_name: some.property.name
property_value: some.property.value`,
			expectedPropertyFilter: PropertyFilter{
				DimensionName:  mustStringFilter(t, "some.dimension.name"),
				DimensionValue: mustStringFilter(t, "some.dimension.value"),
				PropertyName:   mustStringFilter(t, "some.property.name"),
				PropertyValue:  mustStringFilter(t, "some.property.value"),
			},
		},
		{
			name: "default",
			yaml: "",
			expectedPropertyFilter: PropertyFilter{
				DimensionName:  nil,
				DimensionValue: nil,
				PropertyName:   nil,
				PropertyValue:  nil,
			},
		},
		{
			name: "regexes",
			yaml: `dimension_name: /dimension.name/
dimension_value: '!/dimension.value/'
property_name: /property.name/
property_value: '!/property.value/'`,
			expectedPropertyFilter: PropertyFilter{
				DimensionName:  mustStringFilter(t, "/dimension.name/"),
				DimensionValue: mustStringFilter(t, "!/dimension.value/"),
				PropertyName:   mustStringFilter(t, "/property.name/"),
				PropertyValue:  mustStringFilter(t, "!/property.value/"),
			},
		},
		{
			name:          "invalid regex",
			yaml:          "dimension_name: '/(?=not.in.re2)/'",
			expectedError: "1 error(s) decoding:\n\n* error decoding 'dimension_name': error parsing regexp: invalid or unsupported Perl syntax: `(?=`",
		},
		{
			name:          "invalid glob",
			yaml:          "dimension_value: '*[c-a]'",
			expectedError: "1 error(s) decoding:\n\n* error decoding 'dimension_value': hi character 'a' should be greater than lo 'c'",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var conf map[string]interface{}
			err := yaml.Unmarshal([]byte(test.yaml), &conf)
			require.NoError(t, err)

			cm := confmap.NewFromStringMap(conf)
			pf := &PropertyFilter{}
			err = cm.Unmarshal(pf, confmap.WithErrorUnused())
			if test.expectedError != "" {
				require.EqualError(t, err, test.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedPropertyFilter, *pf)
			}
		})
	}
}

func mustStringFilter(t *testing.T, in string) *StringFilter {
	sf, err := NewStringFilter([]string{in})
	require.NoError(t, err)
	return sf
}
