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

package batch_test

import (
	"testing"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
)

func TestBatchingMessages(t *testing.T) {
	t.Parallel()

	b := batch.New()
	for i := 0; i < 948; i++ {
		assert.NoError(t, b.AddProtobufV1(new(empty.Empty), "fixed-string"), "Must not error when adding elements into the batch")
	}

	assert.Len(t, b.Chunk(), 2, "Must have split the batch into two chunks")
	assert.Len(t, b.Chunk(), 2, "Must not modify the stored data within the batch")

	assert.Error(t, b.AddProtobufV1(nil, "fixed-string"), "Must error when invalid record provided")
	assert.Error(t, b.AddProtobufV1(new(empty.Empty), ""), "Must error when invalid partition key provided")
}

func TestCustomBatchSizeConstraints(t *testing.T) {
	t.Parallel()

	b := batch.New(
		batch.WithMaxRecordsPerBatch(1),
	)
	const records = 203
	for i := 0; i < records; i++ {
		assert.NoError(t, b.AddProtobufV1(new(empty.Empty), "fixed-string"), "Must not error when adding elements into the batch")
	}
	assert.Len(t, b.Chunk(), records, "Must have one batch per record added")
}

func BenchmarkChunkingRecords(b *testing.B) {
	bt := batch.New()
	for i := 0; i < 948; i++ {
		assert.NoError(b, bt.AddProtobufV1(new(empty.Empty), "fixed-string"), "Must not error when adding elements into the batch")
	}
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		assert.Len(b, bt.Chunk(), 2, "Must have exactly two chunks")
	}
}
