// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"
)

// BenchmarkS3ContentBuffer measures object preparation without S3 request latency.
func BenchmarkS3ContentBuffer(b *testing.B) {
	for _, count := range []int{100, 1000} {
		logs := plog.NewLogs()
		records := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
		rng := rand.New(rand.NewPCG(1, 2))
		for i := range count {
			record := records.AppendEmpty()
			record.SetTimestamp(1704067200000000000 + pcommon.Timestamp(i)*1000000)
			record.Body().SetStr("completed HTTP request")
			record.Attributes().PutStr("http.request.method", "GET")
			record.Attributes().PutInt("http.response.status_code", 200)
			record.Attributes().PutStr("request.id", fmt.Sprintf("%016x", rng.Uint64()))
		}
		payload, err := (&plog.JSONMarshaler{}).MarshalLogs(logs)
		require.NoError(b, err)

		for _, codec := range []struct {
			typ   configcompression.Type
			level configcompression.Level
		}{
			{typ: "none"},
			{typ: configcompression.TypeGzip, level: configcompression.DefaultCompressionLevel},
			{typ: configcompression.TypeZstd, level: 3},
		} {
			b.Run(fmt.Sprintf("%s/%d_records", codec.typ, count), func(b *testing.B) {
				var opts []ManagerOpt
				if codec.typ.IsCompressed() {
					writer, err := xcompression.NewWriterFactory().NewWriter(codec.typ, configcompression.CompressionParams{Level: codec.level})
					require.NoError(b, err)
					opts = append(opts, WithCompressionWriter(writer))
				}
				builder := &PartitionKeyBuilder{Compression: codec.typ}
				manager := NewS3Manager(zap.NewNop(), "bucket", builder, s3.New(s3.Options{}), "STANDARD", opts...)
				writer := manager.(*s3manager)

				b.SetBytes(int64(len(payload)))
				b.ReportAllocs()
				var size int
				for b.Loop() {
					buf, err := writer.contentBuffer(payload)
					if err != nil {
						b.Fatal(err)
					}
					size = buf.Len()
				}
				b.ReportMetric(float64(size), "output-B/op")
			})
		}
	}
}
