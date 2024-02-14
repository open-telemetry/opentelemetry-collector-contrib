// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
