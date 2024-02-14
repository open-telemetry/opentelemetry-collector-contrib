// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package producer_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/producer"
)

func benchXEmptyMessages(b *testing.B, msgCount int) {
	producer, err := producer.NewBatcher(SetPutRecordsOperation(SuccessfulPutRecordsOperation), "benchmark-stream",
		producer.WithLogger(zaptest.NewLogger(b)),
	)

	require.NoError(b, err, "Must have a valid producer")

	bt := batch.New()
	for i := 0; i < msgCount; i++ {
		assert.NoError(b, bt.AddRecord([]byte("foobar"), "fixed-key"))
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		assert.NoError(b, producer.Put(context.Background(), bt))
	}
}

func BenchmarkEmptyMessages_X100(b *testing.B) {
	benchXEmptyMessages(b, 100)
}

func BenchmarkEmptyMessages_X500(b *testing.B) {
	benchXEmptyMessages(b, 500)
}

func BenchmarkEmptyMessages_X1000(b *testing.B) {
	benchXEmptyMessages(b, 1000)
}
