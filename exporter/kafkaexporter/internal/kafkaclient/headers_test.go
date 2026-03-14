// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
)

func TestMetadataToHeaders(t *testing.T) {
	tests := []struct {
		name     string
		makeCtx  func(t *testing.T) context.Context
		keys     []string
		expected []kgo.RecordHeader
	}{
		{
			name: "nil_keys",
			makeCtx: func(t *testing.T) context.Context {
				return client.NewContext(t.Context(), client.Info{
					Metadata: client.NewMetadata(map[string][]string{"x-tenant": {"t1"}}),
				})
			},
			keys:     nil,
			expected: nil,
		},
		{
			name: "empty_keys",
			makeCtx: func(t *testing.T) context.Context {
				return client.NewContext(t.Context(), client.Info{
					Metadata: client.NewMetadata(map[string][]string{"x-tenant": {"t1"}}),
				})
			},
			keys:     []string{},
			expected: nil,
		},
		{
			name: "no_match",
			makeCtx: func(t *testing.T) context.Context {
				return client.NewContext(t.Context(), client.Info{
					Metadata: client.NewMetadata(map[string][]string{"x-other": {"value"}}),
				})
			},
			keys:     []string{"x-tenant", "x-request-id"},
			expected: nil,
		},
		{
			name: "no_client_info_in_context",
			makeCtx: func(t *testing.T) context.Context {
				return t.Context()
			},
			keys:     []string{"x-tenant"},
			expected: nil,
		},
		{
			name: "all_match_single_values",
			makeCtx: func(t *testing.T) context.Context {
				return client.NewContext(t.Context(), client.Info{
					Metadata: client.NewMetadata(map[string][]string{
						"x-tenant":     {"tenant-1"},
						"x-request-id": {"req-42"},
					}),
				})
			},
			keys: []string{"x-tenant", "x-request-id"},
			expected: []kgo.RecordHeader{
				{Key: "x-tenant", Value: []byte("tenant-1")},
				{Key: "x-request-id", Value: []byte("req-42")},
			},
		},
		{
			name: "all_match_multi_values",
			makeCtx: func(t *testing.T) context.Context {
				return client.NewContext(t.Context(), client.Info{
					Metadata: client.NewMetadata(map[string][]string{
						"x-ids": {"id-1", "id-2", "id-3"},
					}),
				})
			},
			keys: []string{"x-ids"},
			expected: []kgo.RecordHeader{
				{Key: "x-ids", Value: []byte("id-1")},
				{Key: "x-ids", Value: []byte("id-2")},
				{Key: "x-ids", Value: []byte("id-3")},
			},
		},
		{
			name: "partial_match",
			makeCtx: func(t *testing.T) context.Context {
				return client.NewContext(t.Context(), client.Info{
					Metadata: client.NewMetadata(map[string][]string{
						"x-tenant": {"tenant-1"},
						"x-other":  {"ignored"},
					}),
				})
			},
			keys: []string{"x-tenant", "x-request-id"},
			expected: []kgo.RecordHeader{
				{Key: "x-tenant", Value: []byte("tenant-1")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := metadataToHeaders(tt.makeCtx(t), tt.keys)
			if tt.expected == nil {
				require.Nil(t, got)
			} else {
				require.ElementsMatch(t, tt.expected, got)
			}
		})
	}
}

func TestSetMessageHeaders(t *testing.T) {
	tests := []struct {
		name     string
		messages []*kgo.Record
		makeCtx  func(t *testing.T) context.Context
		keys     []string
		expected [][]kgo.RecordHeader // expected headers per message after call
	}{
		{
			name:     "nil_metadata_keys",
			messages: []*kgo.Record{{}},
			makeCtx: func(t *testing.T) context.Context {
				return t.Context()
			},
			keys:     nil,
			expected: [][]kgo.RecordHeader{nil},
		},
		{
			name:     "empty_messages",
			messages: nil,
			makeCtx: func(t *testing.T) context.Context {
				return client.NewContext(t.Context(), client.Info{
					Metadata: client.NewMetadata(map[string][]string{"k": {"v"}}),
				})
			},
			keys:     []string{"k"},
			expected: nil,
		},
		{
			name:     "sets_on_empty_message_headers",
			messages: []*kgo.Record{{}, {}},
			makeCtx: func(t *testing.T) context.Context {
				return client.NewContext(t.Context(), client.Info{
					Metadata: client.NewMetadata(map[string][]string{"x-tenant": {"t1"}}),
				})
			},
			keys: []string{"x-tenant"},
			expected: [][]kgo.RecordHeader{
				{{Key: "x-tenant", Value: []byte("t1")}},
				{{Key: "x-tenant", Value: []byte("t1")}},
			},
		},
		{
			name: "appends_to_existing_headers",
			messages: []*kgo.Record{
				{Headers: []kgo.RecordHeader{{Key: "existing", Value: []byte("val")}}},
			},
			makeCtx: func(t *testing.T) context.Context {
				return client.NewContext(t.Context(), client.Info{
					Metadata: client.NewMetadata(map[string][]string{"x-new": {"new-val"}}),
				})
			},
			keys: []string{"x-new"},
			expected: [][]kgo.RecordHeader{
				{
					{Key: "existing", Value: []byte("val")},
					{Key: "x-new", Value: []byte("new-val")},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setMessageHeaders(tt.makeCtx(t), tt.messages, tt.keys)
			for i, m := range tt.messages {
				require.Equal(t, tt.expected[i], m.Headers)
			}
		})
	}
}
