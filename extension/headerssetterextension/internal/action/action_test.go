// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package action

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsertAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key              string
		value            string
		header           http.Header
		metadata         map[string]string
		expectedHeader   http.Header
		expectedMetadata map[string]string
	}{
		{
			key:              "Key",
			value:            "value",
			header:           http.Header{},
			metadata:         map[string]string{},
			expectedHeader:   map[string][]string{"Key": {"value"}},
			expectedMetadata: map[string]string{"Key": "value"},
		},
		{
			key:              "Key",
			value:            "value",
			header:           map[string][]string{"AnotherKey": {"value"}},
			metadata:         map[string]string{"AnotherKey": "value"},
			expectedHeader:   http.Header{"AnotherKey": []string{"value"}, "Key": []string{"value"}},
			expectedMetadata: map[string]string{"AnotherKey": "value", "Key": "value"},
		},
		{
			key:              "Key",
			value:            "value",
			header:           map[string][]string{"Key": {"different value"}},
			metadata:         map[string]string{"Key": "different value"},
			expectedHeader:   map[string][]string{"Key": {"different value"}},
			expectedMetadata: map[string]string{"Key": "different value"},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			action := Insert{Key: tt.key}

			action.ApplyOnHeaders(tt.header, tt.value)
			action.ApplyOnMetadata(tt.metadata, tt.value)

			assert.Equal(t, tt.expectedHeader, tt.header)
			assert.Equal(t, tt.expectedMetadata, tt.metadata)
		})
	}
}

func TestUpdateAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key              string
		value            string
		header           http.Header
		metadata         map[string]string
		expectedHeader   http.Header
		expectedMetadata map[string]string
	}{
		{
			key:              "Key",
			value:            "value",
			header:           http.Header{},
			metadata:         map[string]string{},
			expectedHeader:   map[string][]string{},
			expectedMetadata: map[string]string{},
		},
		{
			key:              "Key",
			value:            "value",
			header:           map[string][]string{"AnotherKey": {"value"}},
			metadata:         map[string]string{"AnotherKey": "value"},
			expectedHeader:   http.Header{"AnotherKey": []string{"value"}},
			expectedMetadata: map[string]string{"AnotherKey": "value"},
		},
		{
			key:              "Key",
			value:            "value",
			header:           map[string][]string{"Key": {"different value"}},
			metadata:         map[string]string{"Key": "different value"},
			expectedHeader:   map[string][]string{"Key": {"value"}},
			expectedMetadata: map[string]string{"Key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			action := Update{Key: tt.key}

			action.ApplyOnHeaders(tt.header, tt.value)
			action.ApplyOnMetadata(tt.metadata, tt.value)

			assert.Equal(t, tt.expectedHeader, tt.header)
			assert.Equal(t, tt.expectedMetadata, tt.metadata)
		})
	}
}

func TestUpsertAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key              string
		value            string
		header           http.Header
		metadata         map[string]string
		expectedHeader   http.Header
		expectedMetadata map[string]string
	}{
		{
			key:              "Key",
			value:            "value",
			header:           http.Header{},
			metadata:         map[string]string{},
			expectedHeader:   map[string][]string{"Key": {"value"}},
			expectedMetadata: map[string]string{"Key": "value"},
		},
		{
			key:              "Key",
			value:            "value",
			header:           map[string][]string{"AnotherKey": {"value"}},
			metadata:         map[string]string{"AnotherKey": "value"},
			expectedHeader:   http.Header{"AnotherKey": []string{"value"}, "Key": []string{"value"}},
			expectedMetadata: map[string]string{"AnotherKey": "value", "Key": "value"},
		},
		{
			key:              "Key",
			value:            "value",
			header:           map[string][]string{"Key": {"different value"}},
			metadata:         map[string]string{"Key": "different value"},
			expectedHeader:   map[string][]string{"Key": {"value"}},
			expectedMetadata: map[string]string{"Key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			action := Upsert{Key: tt.key}

			action.ApplyOnHeaders(tt.header, tt.value)
			action.ApplyOnMetadata(tt.metadata, tt.value)

			assert.Equal(t, tt.expectedHeader, tt.header)
			assert.Equal(t, tt.expectedMetadata, tt.metadata)
		})
	}
}

func TestDeleteAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key              string
		header           http.Header
		metadata         map[string]string
		expectedHeader   http.Header
		expectedMetadata map[string]string
	}{
		{
			key:              "Key",
			header:           http.Header{},
			metadata:         map[string]string{},
			expectedHeader:   map[string][]string{},
			expectedMetadata: map[string]string{},
		},
		{
			key:              "Key",
			header:           map[string][]string{"AnotherKey": {"value"}, "Key": {"value"}},
			metadata:         map[string]string{"AnotherKey": "value", "Key": "value"},
			expectedHeader:   http.Header{"AnotherKey": []string{"value"}},
			expectedMetadata: map[string]string{"AnotherKey": "value"},
		},
		{
			key:              "Key",
			header:           map[string][]string{"Key": {"different value"}},
			metadata:         map[string]string{"Key": "different value"},
			expectedHeader:   map[string][]string{},
			expectedMetadata: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			action := Delete{Key: tt.key}

			action.ApplyOnHeaders(tt.header, "")
			action.ApplyOnMetadata(tt.metadata, "")

			assert.Equal(t, tt.expectedHeader, tt.header)
			assert.Equal(t, tt.expectedMetadata, tt.metadata)
		})
	}
}
