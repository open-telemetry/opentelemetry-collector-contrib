// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkarequest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
)

func rec(topic string, key, value []byte, headers ...kgo.RecordHeader) *kgo.Record {
	return &kgo.Record{Topic: topic, Key: key, Value: value, Headers: headers}
}

func TestBytesSize(t *testing.T) {
	tests := []struct {
		name    string
		records []*kgo.Record
		want    int
	}{
		{"nil", nil, 0},
		{"empty", []*kgo.Record{}, 0},
		{"single", []*kgo.Record{rec("t", []byte("k"), []byte("vv"))}, 3},
		{
			"with_headers",
			[]*kgo.Record{rec("t", []byte("k"), []byte("vv"), kgo.RecordHeader{Key: "h", Value: []byte("hh")})},
			6,
		},
		{
			"many",
			[]*kgo.Record{
				rec("t", []byte("k1"), []byte("v1")),
				rec("t", nil, []byte("vv")),
			},
			6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, New(tt.records).BytesSize())
		})
	}
}

func TestItemsCount(t *testing.T) {
	assert.Equal(t, 0, New(nil).ItemsCount())
	assert.Equal(t, 3, New([]*kgo.Record{{}, {}, {}}).ItemsCount())
}

func TestRecordsAccessor(t *testing.T) {
	r := []*kgo.Record{rec("t", nil, []byte("v"))}
	assert.Same(t, &r[0], &New(r).Records()[0])
}

func TestOnErrorReturnsSelf(t *testing.T) {
	r := New([]*kgo.Record{rec("t", nil, []byte("v"))})
	assert.Same(t, r, r.OnError(errors.New("anything")))
}

func TestMergeSplit_NoSplit(t *testing.T) {
	r := New([]*kgo.Record{rec("t", nil, []byte("hello"))})
	got, err := r.MergeSplit(t.Context(), 0, exporterhelper.RequestSizerTypeBytes, nil)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 5, got[0].BytesSize())
}

func TestMergeSplit_MergesOther(t *testing.T) {
	a := New([]*kgo.Record{rec("t", nil, []byte("aa"))})
	b := New([]*kgo.Record{rec("t", nil, []byte("bbbb"))})
	got, err := a.MergeSplit(t.Context(), 0, exporterhelper.RequestSizerTypeBytes, b)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 6, got[0].BytesSize())
}

func TestMergeSplit_RejectsIncompatibleType(t *testing.T) {
	r := New(nil)
	_, err := r.MergeSplit(t.Context(), 100, exporterhelper.RequestSizerTypeBytes, fakeRequest{})
	require.Error(t, err)
}

func TestMergeSplit_RejectsUnknownSizer(t *testing.T) {
	r := New([]*kgo.Record{rec("t", nil, []byte("v"))})
	_, err := r.MergeSplit(t.Context(), 100, exporterhelper.RequestSizerType{}, nil)
	require.Error(t, err)
}

func TestMergeSplit_ByItems(t *testing.T) {
	records := []*kgo.Record{
		rec("t", nil, []byte("1")),
		rec("t", nil, []byte("2")),
		rec("t", nil, []byte("3")),
		rec("t", nil, []byte("4")),
		rec("t", nil, []byte("5")),
	}
	got, err := New(records).MergeSplit(t.Context(), 2, exporterhelper.RequestSizerTypeItems, nil)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, 2, got[0].ItemsCount())
	assert.Equal(t, 2, got[1].ItemsCount())
	assert.Equal(t, 1, got[2].ItemsCount())
}

func TestMergeSplit_ByRequests(t *testing.T) {
	r := New([]*kgo.Record{rec("t", nil, []byte("v1")), rec("t", nil, []byte("v2"))})
	got, err := r.MergeSplit(t.Context(), 10, exporterhelper.RequestSizerTypeRequests, nil)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 2, got[0].ItemsCount())
}

func TestMergeSplit_ByBytes_PacksAndOrders(t *testing.T) {
	records := []*kgo.Record{
		rec("t", nil, []byte("aaaa")), // 4
		rec("t", nil, []byte("bb")),   // 2
		rec("t", nil, []byte("ccc")),  // 3
		rec("t", nil, []byte("d")),    // 1
	}
	got, err := New(records).MergeSplit(t.Context(), 5, exporterhelper.RequestSizerTypeBytes, nil)
	require.NoError(t, err)
	require.NotEmpty(t, got)
	for i := range got {
		assert.LessOrEqual(t, got[i].BytesSize(), 5, "bin %d exceeds maxSize", i)
	}
	assertLastIsSmallest(t, got)
}

func TestMergeSplit_ByBytes_OversizedRecordEmittedAlone(t *testing.T) {
	records := []*kgo.Record{
		rec("t", nil, []byte("aa")),               // 2, fits
		rec("t", nil, []byte("oversizedpayload")), // 16, alone
		rec("t", nil, []byte("b")),                // 1, fits
	}
	got, err := New(records).MergeSplit(t.Context(), 5, exporterhelper.RequestSizerTypeBytes, nil)
	require.NoError(t, err)

	var foundOversizedSolo bool
	for _, r := range got {
		if r.ItemsCount() == 1 && r.BytesSize() > 5 {
			foundOversizedSolo = true
		}
	}
	assert.True(t, foundOversizedSolo, "expected oversized record in its own request")
	assertLastIsSmallest(t, got)
}

func TestMergeSplit_ByBytes_LastIsSmallest_Property(t *testing.T) {
	sizes := []int{1, 2, 3, 4, 5, 12, 7, 6, 1, 8, 2, 9}
	records := make([]*kgo.Record, len(sizes))
	for i, s := range sizes {
		records[i] = rec("t", nil, make([]byte, s))
	}
	for _, maxSize := range []int{1, 3, 5, 7, 10, 13, 100} {
		t.Run(fmt.Sprintf("maxSize=%d", maxSize), func(t *testing.T) {
			got, err := New(records).MergeSplit(t.Context(), maxSize, exporterhelper.RequestSizerTypeBytes, nil)
			require.NoError(t, err)
			assertLastIsSmallest(t, got)
		})
	}
}

func assertLastIsSmallest(t *testing.T, got []xexporterhelper.Request) {
	t.Helper()
	require.NotEmpty(t, got)
	last := got[len(got)-1].(*Request).BytesSize()
	for i, r := range got {
		assert.LessOrEqualf(t, last, r.(*Request).BytesSize(),
			"last (size=%d) is larger than chunk %d (size=%d)", last, i, r.(*Request).BytesSize())
	}
}

// fakeRequest is an arbitrary xexporterhelper.Request implementation used to
// exercise the type-assertion failure path in MergeSplit.
type fakeRequest struct{}

func (fakeRequest) ItemsCount() int { return 0 }
func (fakeRequest) BytesSize() int  { return 0 }
func (fakeRequest) MergeSplit(_ context.Context, _ int, _ exporterhelper.RequestSizerType, _ xexporterhelper.Request) ([]xexporterhelper.Request, error) {
	return nil, nil
}
