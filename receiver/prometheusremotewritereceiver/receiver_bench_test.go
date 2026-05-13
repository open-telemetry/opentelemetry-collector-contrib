// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gogo/protobuf/proto"
	remoteapi "github.com/prometheus/client_golang/exp/api/remote"
	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
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

// makeWriteV2Request builds a deterministic writev2.Request with the requested number of series,
// samples per series, and extra labels per series.
// It includes a target_info series with a few extra resource-level labels (service_name,
// service_version, cloud_provider) sharing the same job/instance as all regular metrics,
// and otel_scope_name, otel_scope_version, otel_scope_schema_url, and a custom scope attribute
// on every regular metric series.
func makeWriteV2Request(numSeries, samplesPerSeries, extraLabels int) *writev2.Request {
	symbols := []string{
		"",
		"__name__",
		"job",
		"my_job",
		"instance",
		"my_instance",
		"target_info",
		"service_name",
		"my_service",
		"service_version",
		"v1.0",
		"cloud_provider",
		"gcp",
		"otel_scope_name",
		"bench_scope",
		"otel_scope_version",
		"v0.1.0",
		"otel_scope_schema_url",
		"https://example.com/schema",
		"otel_scope_env",
		"prod",
	}

	extraLabelIndexStart := len(symbols)
	for i := range extraLabels {
		symbols = append(symbols, fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	ts := make([]writev2.TimeSeries, 0, numSeries+1)

	// target_info sets resource attributes for all series sharing the same job/instance.
	ts = append(ts, writev2.TimeSeries{
		Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
		LabelsRefs: []uint32{1, 6, 2, 3, 4, 5, 7, 8, 9, 10, 11, 12},
		Samples:    []writev2.Sample{{Value: 1, Timestamp: 1}},
	})

	for i := range numSeries {
		metricName := fmt.Sprintf("metric_%d", i)
		symbols = append(symbols, metricName)
		nameIdx := uint32(len(symbols) - 1)

		labelRefs := []uint32{
			1, nameIdx,
			2, 3,
			4, 5,
			13, 14,
			15, 16,
			17, 18,
			19, 20,
		}
		for j := range extraLabels {
			labelRefs = append(labelRefs, uint32(extraLabelIndexStart+2*j), uint32(extraLabelIndexStart+2*j+1))
		}

		samples := make([]writev2.Sample, 0, samplesPerSeries)
		for s := range samplesPerSeries {
			samples = append(samples, writev2.Sample{Value: float64(1), Timestamp: int64(s + 1), StartTimestamp: int64(s + 1)})
		}

		ts = append(ts, writev2.TimeSeries{
			Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
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

const (
	extraLabelsSize = 10
)

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
					prw := setupMetricsReceiver(b)

					// Precompute payload (raw proto) to focus on receiver translation cost.
					req := makeWriteV2Request(sz, samples, extraLabelsSize)
					payload := encodeProto(req)

					b.ResetTimer()

					var counter atomic.Int64
					b.SetParallelism(conc)
					b.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							counter.Add(1)
							r := httptest.NewRequest(http.MethodPost, "/api/v1/write", bytes.NewReader(payload))
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
