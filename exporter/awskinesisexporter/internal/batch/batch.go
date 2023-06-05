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

package batch // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"

import (
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types" //nolint:staticcheck // Some encoding types uses legacy prototype version
	"go.opentelemetry.io/collector/consumer/consumererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/compress"
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

	compression compress.Compressor

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

func WithCompression(compressor compress.Compressor) Option {
	return func(bt *Batch) {
		if compressor != nil {
			bt.compression = compressor
		}
	}
}

func New(opts ...Option) *Batch {
	bt := &Batch{
		maxBatchSize:  MaxBatchedRecords,
		maxRecordSize: MaxRecordSize,
		compression:   compress.NewNoopCompressor(),
		records:       make([]types.PutRecordsRequestEntry, 0, MaxRecordSize),
	}

	for _, op := range opts {
		op(bt)
	}

	return bt
}

func (b *Batch) AddRecord(raw []byte, key string) error {
	record, err := b.compression.Do(raw)
	if err != nil {
		return err
	}

	if l := len(key); l == 0 || l > 256 {
		return ErrPartitionKeyLength
	}

	if l := len(record); l == 0 || l > b.maxRecordSize {
		return ErrRecordLength
	}

	b.records = append(b.records, types.PutRecordsRequestEntry{
		Data:         record,
		PartitionKey: aws.String(key),
	})
	return nil
}

// Chunk breaks up the iternal queue into blocks that can be used
// to be written to he kinesis.PutRecords endpoint
func (b *Batch) Chunk() (chunks [][]types.PutRecordsRequestEntry) {
	// Using local copies to avoid mutating internal data
	var (
		slice = b.records
		size  = b.maxBatchSize
	)
	for len(slice) != 0 {
		if len(slice) < size {
			size = len(slice)
		}
		chunks = append(chunks, slice[0:size])
		slice = slice[size:]
	}
	return chunks
}
