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

func TestResourceFieldGet(t *testing.T) {
	cases := []struct {
		name       string
		resources  map[string]interface{}
		field      Field
		expected   interface{}
		expectedOK bool
	}{
		{
			"Simple",
			map[string]interface{}{
				"test": "val",
			},
			NewResourceField("test"),
			"val",
			true,
		},
		{
			"NonexistentKey",
			map[string]interface{}{
				"test": "val",
			},
			NewResourceField("nonexistent"),
			nil,
			false,
		},
		{
			"NilMap",
			nil,
			NewResourceField("nonexistent"),
			"",
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Resource = tc.resources
			val, ok := entry.Get(tc.field)
			require.Equal(t, tc.expectedOK, ok)
			require.Equal(t, tc.expected, val)
		})
	}
}

func TestResourceFieldDelete(t *testing.T) {
	cases := []struct {
		name              string
		resources         map[string]interface{}
		field             Field
		expected          interface{}
		expectedOK        bool
		expectedResources map[string]interface{}
	}{
		{
			"Simple",
			map[string]interface{}{
				"test": "val",
			},
			NewResourceField("test"),
			"val",
			true,
			map[string]interface{}{},
		},
		{
			"NonexistentKey",
			map[string]interface{}{
				"test": "val",
			},
			NewResourceField("nonexistent"),
			nil,
			false,
			map[string]interface{}{
				"test": "val",
			},
		},
		{
			"NilMap",
			nil,
			NewResourceField("nonexistent"),
			"",
			false,
			nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := New()
			entry.Resource = tc.resources
			val, ok := entry.Delete(tc.field)
			require.Equal(t, tc.expectedOK, ok)
			require.Equal(t, tc.expected, val)
		})
	}
}

func TestResourceFieldSet(t *testing.T) {
	cases := []struct {
		name        string
		resources   map[string]interface{}
		field       Field
		val         interface{}
		expected    map[string]interface{}
		expectedErr bool
	}{
		{
			"Simple",
			map[string]interface{}{},
			NewResourceField("test"),
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
			NewResourceField("test"),
			"val",
			map[string]interface{}{
				"test": "val",
			},
			false,
		},
		{
			"NilMap",
			nil,
			NewResourceField("test"),
			"val",
			map[string]interface{}{
				"test": "val",
			},
			false,
		},
		{
			"NonString",
			map[string]interface{}{},
			NewResourceField("test"),
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
			entry.Resource = tc.resources
			err := entry.Set(tc.field, tc.val)
			if tc.expectedErr {
				require.Error(t, err)
				return
			}

			require.Equal(t, tc.expected, entry.Resource)
		})
	}
}

func TestResourceFieldString(t *testing.T) {
	cases := []struct {
		name     string
		field    ResourceField
		expected string
	}{
		{
			"Simple",
			ResourceField{"foo"},
			"resource.foo",
		},
		{
			"Empty",
			ResourceField{""},
			"resource.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.field.String())
		})
	}
}
