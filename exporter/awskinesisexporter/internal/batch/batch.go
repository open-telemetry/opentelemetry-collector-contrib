// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batch // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"

import (
	"bytes"
	"errors"
	"iter"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"go.opentelemetry.io/collector/consumer/consumererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"
)

const (
	MaxRecordSize     = 1 << 20 // 1MiB
	MaxBatchedRecords = 500
)

var (
	// ErrPartitionKeyLength is used when the given key exceeds the allowed kinesis limit of 256 characters
	ErrPartitionKeyLength = errors.New("partition key size is greater than 256 characters")
	// ErrRecordLength is used when attempted record results in a byte array greater than 1MiB
	ErrRecordLength = consumererror.NewPermanent(errors.New("record size is greater than 1 MiB"))
)

type Batch struct {
	maxBatchSize  int
	maxRecordSize int

	writer xcompression.Writer

	records []types.PutRecordsRequestEntry
}

type Option func(bt *Batch)

func WithMaxRecordsPerBatch(limit int) Option {
	return func(bt *Batch) {
		if MaxBatchedRecords < limit {
			limit = MaxBatchedRecords
		}
		bt.maxBatchSize = limit
	}
}

func WithMaxRecordSize(size int) Option {
	return func(bt *Batch) {
		if MaxRecordSize < size {
			size = MaxRecordSize
		}
		bt.maxRecordSize = size
	}
}

func WithCompressionWriter(w xcompression.Writer) Option {
	return func(bt *Batch) {
		bt.writer = w
	}
}

func New(opts ...Option) *Batch {
	bt := &Batch{
		maxBatchSize:  MaxBatchedRecords,
		maxRecordSize: MaxRecordSize,
		writer:        xcompression.Nop{},
		records:       make([]types.PutRecordsRequestEntry, 0, MaxBatchedRecords),
	}

	for _, op := range opts {
		op(bt)
	}

	return bt
}

func (b *Batch) AddRecord(raw []byte, key string) error {
	data := raw
	switch b.writer.(type) {
	case xcompression.Nop, *xcompression.Nop:
		// In the event that no compression is set,
		// it means that the raw can be passed
		// through directly without modification.
	default:
		buf := bytes.NewBuffer(nil)
		if _, err := b.writer.CompressBytes(buf, raw); err != nil {
			return err
		}
		data = buf.Bytes()
	}

	if l := len(key); l == 0 || l > 256 {
		return ErrPartitionKeyLength
	}

	if l := len(data); l == 0 || l > b.maxRecordSize {
		return ErrRecordLength
	}

	b.records = append(b.records, types.PutRecordsRequestEntry{
		Data:         data,
		PartitionKey: aws.String(key),
	})
	return nil
}

// Chunk breaks up the internal queue into blocks that can be used
// to be written to he kinesis.PutRecords endpoint
func (b *Batch) Chunk() iter.Seq[[]types.PutRecordsRequestEntry] {
	return slices.Chunk(b.records, b.maxBatchSize)
}
