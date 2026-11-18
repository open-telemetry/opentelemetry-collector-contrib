// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batch_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/compress"
)

func TestCompressedRecordSizeError(t *testing.T) {
	t.Parallel()

	raw := bytes.Repeat([]byte("private record contents"), 1000)
	compressor, err := compress.NewCompressor("gzip")
	require.NoError(t, err)
	encoded, err := compressor(raw)
	require.NoError(t, err)
	require.Less(t, len(encoded), len(raw))

	limit := len(encoded) - 1
	b := batch.New(batch.WithCompressionType("gzip"), batch.WithMaxRecordSize(limit))
	err = b.AddRecord(raw, "private-partition-key")
	require.ErrorIs(t, err, batch.ErrRecordLength)
	assert.True(t, consumererror.IsPermanent(err))
	assert.EqualError(t, err, fmt.Sprintf("%s: encoded_size=%d bytes, max_record_size=%d bytes", batch.ErrRecordLength, len(encoded), limit))
}

func TestBatchingMessages(t *testing.T) {
	t.Parallel()

	b := batch.New()
	for range 948 {
		assert.NoError(t, b.AddRecord([]byte("foobar"), "fixed-string"), "Must not error when adding elements into the batch")
	}

	chunk := b.Chunk()
	for _, records := range chunk {
		for _, record := range records {
			assert.Equal(t, []byte("foobar"), record.Data, "Must have the expected record value")
			assert.Equal(t, "fixed-string", *record.PartitionKey, "Must have the expected partition key")
		}
	}

	assert.Len(t, chunk, 2, "Must have split the batch into two chunks")
	assert.Len(t, chunk, 2, "Must not modify the stored data within the batch")

	assert.Error(t, b.AddRecord(nil, "fixed-string"), "Must error when invalid record provided")
	assert.Error(t, b.AddRecord([]byte("some data that is very important"), ""), "Must error when invalid partition key provided")
}

func TestCustomBatchSizeConstraints(t *testing.T) {
	t.Parallel()

	b := batch.New(
		batch.WithMaxRecordsPerBatch(1),
	)
	const records = 203
	for range records {
		assert.NoError(t, b.AddRecord([]byte("foobar"), "fixed-string"), "Must not error when adding elements into the batch")
	}
	assert.Len(t, b.Chunk(), records, "Must have one batch per record added")
}

func BenchmarkChunkingRecords(b *testing.B) {
	bt := batch.New()
	for range 948 {
		assert.NoError(b, bt.AddRecord([]byte("foobar"), "fixed-string"), "Must not error when adding elements into the batch")
	}
	b.ReportAllocs()

	for b.Loop() {
		assert.Len(b, bt.Chunk(), 2, "Must have exactly two chunks")
	}
}
