// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package producer_test

import (
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
	for range msgCount {
		assert.NoError(b, bt.AddRecord([]byte("foobar"), "fixed-key"))
	}

	b.ReportAllocs()

	for b.Loop() {
		assert.NoError(b, producer.Put(b.Context(), bt))
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
