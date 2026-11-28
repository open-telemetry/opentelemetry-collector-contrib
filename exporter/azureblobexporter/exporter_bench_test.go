// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

func BenchmarkBlobCompression(b *testing.B) {
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

		for _, compression := range []configcompression.Type{"none", configcompression.TypeGzip, configcompression.TypeZstd} {
			b.Run(fmt.Sprintf("%s/%d_records", compression, count), func(b *testing.B) {
				cfg := createDefaultConfig().(*Config)
				cfg.Auth.ConnectionString = "DefaultEndpointsProtocol=https;AccountName=fake;AccountKey=ZmFrZQ==;EndpointSuffix=core.windows.net"
				cfg.Compression = compression
				exporter, createErr := newAzureBlobExporter(cfg, zap.NewNop(), pipeline.SignalLogs)
				require.NoError(b, createErr)
				require.NoError(b, exporter.start(b.Context(), componenttest.NewNopHost()))
				b.SetBytes(int64(len(payload)))
				b.ReportAllocs()
				var size int
				for b.Loop() {
					compressed, err := exporter.compressContent(payload)
					if err != nil {
						b.Fatal(err)
					}
					size = len(compressed)
				}
				b.ReportMetric(float64(size), "output-B/op")
			})
		}
	}
}
