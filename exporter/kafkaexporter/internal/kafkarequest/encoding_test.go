// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkarequest

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
)

func TestEncoding_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		records []*kgo.Record
	}{
		{
			"empty",
			nil,
		},
		{
			"single_minimal",
			[]*kgo.Record{{Topic: "t"}},
		},
		{
			"single_full",
			[]*kgo.Record{
				{
					Topic:     "my-topic",
					Key:       []byte("the-key"),
					Value:     []byte("the-value"),
					Partition: 7,
					Headers: []kgo.RecordHeader{
						{Key: "h1", Value: []byte("v1")},
						{Key: "h2", Value: []byte("")},
					},
				},
			},
		},
		{
			"many",
			[]*kgo.Record{
				{Topic: "a", Key: []byte("k"), Value: []byte("v")},
				{Topic: "b", Value: []byte("only-value")},
				{Topic: "c", Key: []byte("only-key")},
			},
		},
		{
			"nil_vs_empty_normalized_to_nil",
			[]*kgo.Record{{Topic: "t", Key: nil, Value: nil}},
		},
	}
	enc := NewEncoding()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := enc.Marshal(t.Context(), New(tt.records))
			require.NoError(t, err)

			_, got, err := enc.Unmarshal(data)
			require.NoError(t, err)

			gotReq, ok := got.(*Request)
			require.True(t, ok)
			require.Len(t, gotReq.records, len(tt.records))
			for i, want := range tt.records {
				assertRecordEqual(t, want, gotReq.records[i])
			}
		})
	}
}

func TestEncoding_BytesSizePreserved(t *testing.T) {
	records := []*kgo.Record{
		{Topic: "t", Key: []byte("k"), Value: []byte("vv")},
		{Topic: "t", Value: []byte("xxxxx"), Headers: []kgo.RecordHeader{{Key: "h", Value: []byte("hv")}}},
	}
	req := New(records)
	originalSize := req.BytesSize()

	data, err := NewEncoding().Marshal(t.Context(), req)
	require.NoError(t, err)
	_, got, err := NewEncoding().Unmarshal(data)
	require.NoError(t, err)

	assert.Equal(t, originalSize, got.BytesSize())
}

func TestEncoding_Marshal_RejectsWrongType(t *testing.T) {
	_, err := NewEncoding().Marshal(t.Context(), fakeRequest{})
	require.Error(t, err)
}

func TestEncoding_Unmarshal_UnknownVersion(t *testing.T) {
	buf := binary.AppendUvarint(nil, 999) // unknown version
	buf = binary.AppendUvarint(buf, 0)    // record count
	_, _, err := NewEncoding().Unmarshal(buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestEncoding_Unmarshal_EmptyInput(t *testing.T) {
	_, _, err := NewEncoding().Unmarshal(nil)
	require.Error(t, err)
}

func TestEncoding_Unmarshal_TrailingBytes(t *testing.T) {
	data, err := NewEncoding().Marshal(t.Context(), New([]*kgo.Record{{Topic: "t"}}))
	require.NoError(t, err)
	data = append(data, 0xff, 0xff, 0xff)
	_, _, err = NewEncoding().Unmarshal(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trailing")
}

func TestEncoding_Unmarshal_TruncatedAtEveryByte(t *testing.T) {
	full, err := NewEncoding().Marshal(t.Context(), New([]*kgo.Record{
		{Topic: "t", Key: []byte("k"), Value: []byte("v"), Headers: []kgo.RecordHeader{{Key: "h", Value: []byte("v")}}},
	}))
	require.NoError(t, err)
	require.NotEmpty(t, full)

	// Truncate at every prefix length and ensure no panic and we get an error
	// (except the full payload itself).
	for n := range len(full) {
		_, _, err := NewEncoding().Unmarshal(full[:n])
		require.Error(t, err, "expected error for truncation at %d/%d", n, len(full))
	}
}

func assertRecordEqual(t *testing.T, want, got *kgo.Record) {
	t.Helper()
	assert.Equal(t, want.Topic, got.Topic)
	assertBytesEqual(t, want.Key, got.Key, "key")
	assertBytesEqual(t, want.Value, got.Value, "value")
	assert.Equal(t, want.Partition, got.Partition)
	require.Len(t, got.Headers, len(want.Headers))
	for i, h := range want.Headers {
		assert.Equal(t, h.Key, got.Headers[i].Key, "header[%d].Key", i)
		assertBytesEqual(t, h.Value, got.Headers[i].Value, "header[%d].Value", i)
	}
}

// assertBytesEqual treats nil and empty slices as equivalent (the encoder
// intentionally normalizes empty bytes to nil on decode).
func assertBytesEqual(t *testing.T, want, got []byte, msgAndArgs ...any) {
	t.Helper()
	if len(want) == 0 && len(got) == 0 {
		return
	}
	assert.Equal(t, want, got, msgAndArgs...)
}
