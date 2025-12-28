// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver/internal/metadata"
	remoteapi "github.com/prometheus/client_golang/exp/api/remote"
	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// makeLabels builds a labels.Labels with the given total number of labels.
// It always includes "job", "instance", and the metric name label, plus (total-3) extra labels.
func makeLabels(total int) labels.Labels {
	if total < 3 {
		total = 3
	}
	m := make(map[string]string, total)
	m["job"] = "job"
	m["instance"] = "instance"
	m["__name__"] = "metric"
	for i := 0; i < total-3; i++ {
		m[fmt.Sprintf("k%d", i)] = fmt.Sprintf("v%d", i)
	}
	return labels.FromMap(m)
}

func BenchmarkExtractAttributes(b *testing.B) {
	sizes := []int{5, 20, 100, 500, 1000, 2000}
	for _, sz := range sizes {
		ls := makeLabels(sz)
		b.Run(fmt.Sprintf("%d", sz), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m := extractAttributes(ls)
				_ = m.Len()
			}
		})
	}
}

// setupMetricsReceiverBench creates a receiver instance for benchmarks. It mirrors setupMetricsReceiver
// but accepts testing.B/TB so it can be used from benchmarks.
func setupMetricsReceiverBench(tb testing.TB) *prometheusRemoteWriteReceiver {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	prwReceiverIfc, err := factory.CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	if err != nil {
		tb.Fatalf("failed to create metrics receiver: %v", err)
	}
	prw := prwReceiverIfc.(*prometheusRemoteWriteReceiver)

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.MustNewID("test"),
		Transport:              "http",
		ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type),
	})
	if err != nil {
		tb.Fatalf("failed to create obsreport: %v", err)
	}
	prw.obsrecv = obsrecv

	// Ensure LRU is purged when benchmark finishes
	if tb, ok := tb.(interface{ Cleanup(func()) }); ok {
		tb.Cleanup(func() { prw.rmCache.Purge() })
	}

	return prw
}

// makeWriteV2Request builds a deterministic writev2.Request with the requested number of series,
// samples per series, and extra labels per series.
func makeWriteV2Request(numSeries, samplesPerSeries, extraLabels int) *writev2.Request {
	symbols := []string{"", "__name__", "job", "my_job", "instance", "my_instance"}
	// extra labels
	extraLabelIndexStart := len(symbols)
	for j := 0; j < extraLabels; j++ {
		k := fmt.Sprintf("k%d", j)
		v := fmt.Sprintf("v%d", j)
		symbols = append(symbols, k, v)
	}

	ts := make([]writev2.TimeSeries, 0, numSeries)
	for i := 0; i < numSeries; i++ {
		labelRefs := []uint32{1} // __name__
		// Add a metric name symbol per series to avoid all series being identical
		metricName := fmt.Sprintf("metric_%d", i)
		symbols = append(symbols, metricName)
		labelRefs = append(labelRefs, uint32(len(symbols)-1))
		// job & instance
		labelRefs = append(labelRefs, 2, 3, 4, 5)
		// extra labels
		for j := 0; j < extraLabels; j++ {
			// add refs to key and value
			labelRefs = append(labelRefs, uint32(extraLabelIndexStart+2*j), uint32(extraLabelIndexStart+2*j+1))
		}

		samples := make([]writev2.Sample, 0, samplesPerSeries)
		for s := 0; s < samplesPerSeries; s++ {
			samples = append(samples, writev2.Sample{Value: float64(1), Timestamp: int64(s + 1), StartTimestamp: int64(s + 1)})
		}

		ts = append(ts, writev2.TimeSeries{
			Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE, HelpRef: 0, UnitRef: 0},
			LabelsRefs: labelRefs,
			Samples:    samples,
		})
	}

	return &writev2.Request{Symbols: symbols, Timeseries: ts}
}

func encodeProto(req *writev2.Request) []byte {
	b, _ := proto.Marshal(req)
	return b
}

func BenchmarkRemoteWrite(b *testing.B) {
	seriesSizes := []int{10, 100, 1000}
	samplesList := []int{1, 5}
	concurrency := []int{1, 4, 16}

	for _, sz := range seriesSizes {
		for _, samples := range samplesList {
			for _, conc := range concurrency {
				name := fmt.Sprintf("S%d_Samples%d_C%d", sz, samples, conc)
				b.Run(name, func(b *testing.B) {
					b.ReportAllocs()
					prw := setupMetricsReceiverBench(b)

					// Precompute payload (raw proto) to focus on receiver translation cost.
					req := makeWriteV2Request(sz, samples, 10)
					payload := encodeProto(req)

					// Warmup
					for i := 0; i < 20; i++ {
						r := httptest.NewRequest("POST", "/api/v1/write", bytes.NewReader(payload))
						r.Header.Set("Content-Type", fmt.Sprintf("application/x-protobuf;proto=%s", remoteapi.WriteV2MessageType))
						w := httptest.NewRecorder()
						prw.handlePRW(w, r)
					}

					b.ResetTimer()

					var counter int64
					b.SetParallelism(conc)
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							atomic.AddInt64(&counter, 1)
							r := httptest.NewRequest("POST", "/api/v1/write", bytes.NewReader(payload))
							r.Header.Set("Content-Type", fmt.Sprintf("application/x-protobuf;proto=%s", remoteapi.WriteV2MessageType))
							w := httptest.NewRecorder()
							prw.handlePRW(w, r)
							_ = w.Result().StatusCode
						}
					})
					b.StopTimer()
				})
			}
		}
	}
}
