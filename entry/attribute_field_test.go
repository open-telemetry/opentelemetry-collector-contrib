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

package entry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAttributeFieldGet(t *testing.T) {
	cases := []struct {
		name       string
		attributes map[string]interface{}
		field      Field
		expected   interface{}
		expectedOK bool
	}{
		{
			"Simple",
			map[string]interface{}{
				"test": "val",
			},
			NewAttributeField("test"),
			"val",
			true,
		},
		{
			"NonexistentKey",
			map[string]interface{}{
				"test": "val",
			},
			NewAttributeField("nonexistent"),
			nil,
			false,
		},
		{
			"NilMap",
			nil,
			NewAttributeField("nonexistent"),
			"",
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Attributes = tc.attributes
			val, ok := entry.Get(tc.field)
			require.Equal(t, tc.expectedOK, ok)
			require.Equal(t, tc.expected, val)
		})
	}
}

func TestAttributeFieldDelete(t *testing.T) {
	cases := []struct {
		name               string
		attributes         map[string]interface{}
		field              Field
		expected           interface{}
		expectedOK         bool
		expectedAttributes map[string]interface{}
	}{
		{
			"Simple",
			map[string]interface{}{
				"test": "val",
			},
			NewAttributeField("test"),
			"val",
			true,
			map[string]interface{}{},
		},
		{
			"NonexistentKey",
			map[string]interface{}{
				"test": "val",
			},
			NewAttributeField("nonexistent"),
			nil,
			false,
			map[string]interface{}{
				"test": "val",
			},
		},
		{
			"NilMap",
			nil,
			NewAttributeField("nonexistent"),
			"",
			false,
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Attributes = tc.attributes
			val, ok := entry.Delete(tc.field)
			require.Equal(t, tc.expectedOK, ok)
			require.Equal(t, tc.expected, val)
		})
	}
}

func TestAttributeFieldSet(t *testing.T) {
	cases := []struct {
		name        string
		attributes  map[string]interface{}
		field       Field
		val         interface{}
		expected    map[string]interface{}
		expectedErr bool
	}{
		{
			"Simple",
			map[string]interface{}{},
			NewAttributeField("test"),
			"val",
			map[string]interface{}{
				"test": "val",
			},
			false,
		},
		{
			"Overwrite",
			map[string]interface{}{
				"test": "original",
			},
			NewAttributeField("test"),
			"val",
			map[string]interface{}{
				"test": "val",
			},
			false,
		},
		{
			"NilMap",
			nil,
			NewAttributeField("test"),
			"val",
			map[string]interface{}{
				"test": "val",
			},
			false,
		},
		{
			"NonString",
			map[string]interface{}{},
			NewAttributeField("test"),
			123,
			map[string]interface{}{
				"test": "val",
			},
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Attributes = tc.attributes
			err := entry.Set(tc.field, tc.val)
			if tc.expectedErr {
				require.Error(t, err)
				return
			}

			require.Equal(t, tc.expected, entry.Attributes)
		})
	}
}

func TestAttributeFieldString(t *testing.T) {
	cases := []struct {
		name     string
		field    AttributeField
		expected string
	}{
		{
			"Simple",
			AttributeField{"foo"},
			"attributes.foo",
		},
		{
			"Empty",
			AttributeField{""},
			"attributes.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.field.String())
		})
	}
}
