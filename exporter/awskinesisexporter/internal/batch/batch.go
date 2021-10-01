// Copyright  OpenTelemetry Authors
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

package batch

import (
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kinesis"
	protov1 "github.com/golang/protobuf/proto" //nolint:staticcheck // Some encoding types uses legacy prototype version
	"go.opentelemetry.io/collector/consumer/consumererror"
	protov2 "google.golang.org/protobuf/proto"
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

	records []*kinesis.PutRecordsRequestEntry
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

func New(opts ...Option) *Batch {
	bt := &Batch{
		maxBatchSize:  MaxBatchedRecords,
		maxRecordSize: MaxRecordSize,
		records:       make([]*kinesis.PutRecordsRequestEntry, 0, MaxRecordSize),
	}

	for _, op := range opts {
		op(bt)
	}

	return bt
}

func (b *Batch) addRaw(raw []byte, key string) error {
	if l := len(key); l == 0 || l > 256 {
		return ErrPartitionKeyLength
	}

	if len(raw) > b.maxRecordSize {
		return ErrRecordLength
	}

	b.records = append(b.records, &kinesis.PutRecordsRequestEntry{Data: raw, PartitionKey: aws.String(key)})
	return nil
}

// AddProtobufV1 allows for deprecated protobuf generated types to be exported
// and marshaled into records to be exported to kinesis.
// The protobuf v1 message is considered deprecated so where possible to use
// batch.AddProtobufV2.
func (b *Batch) AddProtobufV1(message protov1.Message, key string) error {
	data, err := protov1.Marshal(message)
	if err != nil {
		return err
	}
	return b.addRaw(data, key)
}

// AddProtobufV2 accepts the protobuf message and marshal it into a record
// ready to submit to kinesis.
func (b *Batch) AddProtobufV2(message protov2.Message, key string) error {
	data, err := protov2.Marshal(message)
	if err != nil {
		return err
	}
	return b.addRaw(data, key)
}

// Chunk breaks up the iternal queue into blocks that can be used
// to be written to he kinesis.PutRecords endpoint
func (b *Batch) Chunk() (chunks [][]*kinesis.PutRecordsRequestEntry) {
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
