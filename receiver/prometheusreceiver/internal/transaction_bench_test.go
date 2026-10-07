// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"testing"

	types "github.com/gogo/protobuf/types"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/model/textparse"
	dto "github.com/prometheus/prometheus/prompb/io/prometheus/client"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/tsdbutil"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	mdata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

const (
	numSeries    = 10000
	protoType    = "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited"
	promTextType = "text/plain; version=0.0.4"
)

var (
	benchTarget = scrape.NewTarget(
		labels.FromMap(map[string]string{
			model.InstanceLabel: "localhost:8080",
			model.JobLabel:      "benchmark",
		}),
		&config.ScrapeConfig{},
		map[model.LabelName]model.LabelValue{
			model.AddressLabel: "localhost:8080",
			model.SchemeLabel:  "http",
		},
		nil,
	)

	benchCtx = scrape.ContextWithTarget(context.Background(), benchTarget)
)

// BenchmarkAppend benchmarks the Append method of the transaction.
// It tests the performance of appending classic metric types (counters, gauges, summaries, histograms).
func BenchmarkAppend(b *testing.B) {
	benchmarkAppend(b)
}

func benchmarkAppend(b *testing.B) {
	labelSets := generateLabelSets(numSeries, 50)
	timestamp := int64(1234567890)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tx := newBenchmarkTransaction(b)
		b.StartTimer()

		for j, ls := range labelSets {
			value := float64(j)
			_, err := tx.Append(0, ls, 0, timestamp, value, nil, nil, storage.AOptions{})
			assert.NoError(b, err)
		}
	}
}

// BenchmarkAppendHistogram benchmarks the AppendHistogram method of the transaction.
// It tests the performance of appending native histogram metrics.
func BenchmarkAppendHistogram(b *testing.B) {
	benchmarkAppendHistogram(b)
}

func benchmarkAppendHistogram(b *testing.B) {
	labelSets := generateLabelSets(numSeries, 50)
	histograms := generateNativeHistograms(numSeries)
	timestamp := int64(1234567890)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tx := newBenchmarkTransaction(b)
		b.StartTimer()

		for j := range labelSets {
			_, err := tx.Append(0, labelSets[j], 0, timestamp, 0, histograms[j], nil, storage.AOptions{})
			assert.NoError(b, err)
		}
	}
}

// BenchmarkCommit benchmarks the Commit method which converts accumulated metrics to pmetrics format
// and delivers them to the consumer. This is separate from Append/AppendHistogram to measure the
// conversion and delivery overhead independently.
// Note: The presence of target_info and otel_scope_info metrics affects the performance of the Commit method,
// so they are benchmarked in sub-benchmarks.
func BenchmarkCommit(b *testing.B) {
	b.Run("ClassicMetrics", func(b *testing.B) {
		b.Run("Baseline", func(b *testing.B) {
			benchmarkCommit(b, false, false, false)
		})

		b.Run("WithTargetInfo", func(b *testing.B) {
			benchmarkCommit(b, false, true, false)
		})

		b.Run("WithScopeInfo", func(b *testing.B) {
			benchmarkCommit(b, false, false, true)
		})
	})

	b.Run("NativeHistogram", func(b *testing.B) {
		b.Run("Baseline", func(b *testing.B) {
			benchmarkCommit(b, true, false, false)
		})

		b.Run("WithTargetInfo", func(b *testing.B) {
			benchmarkCommit(b, true, true, false)
		})

		b.Run("WithScopeInfo", func(b *testing.B) {
			benchmarkCommit(b, true, false, true)
		})
	})
}

func benchmarkCommit(b *testing.B, useNativeHistograms, withTargetInfo, withScopeInfo bool) {
	labelSets := generateLabelSets(numSeries, 50)
	var histograms []*histogram.Histogram
	if useNativeHistograms {
		histograms = generateNativeHistograms(numSeries)
	}
	timestamp := int64(1234567890)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Setup: Create transaction and append all data (not timed)
		b.StopTimer()
		tx := newBenchmarkTransaction(b)

		if withTargetInfo {
			targetInfoLabels := createTargetInfoLabels()
			_, _ = tx.Append(0, targetInfoLabels, 0, timestamp, 1, nil, nil, storage.AOptions{})
		}

		if withScopeInfo {
			scopeInfoLabels := createScopeInfoLabels()
			_, _ = tx.Append(0, scopeInfoLabels, 0, timestamp, 1, nil, nil, storage.AOptions{})
		}

		if useNativeHistograms {
			for j := range labelSets {
				_, _ = tx.Append(0, labelSets[j], 0, timestamp, 0, histograms[j], nil, storage.AOptions{})
			}
		} else {
			for j, ls := range labelSets {
				_, _ = tx.Append(0, ls, 0, timestamp, float64(j), nil, nil, storage.AOptions{})
			}
		}
		b.StartTimer()

		// Benchmark: Only measure Commit
		err := tx.Commit()
		assert.NoError(b, err)
	}
}

// BenchmarkE2ETransaction benchmarks the entire lifecycle of a transaction (new, Append, Commit)
// without stopping the timer, providing a fair end-to-end comparison of memory and CPU performance.
func BenchmarkE2ETransaction(b *testing.B) {
	b.Run("ClassicMetrics", func(b *testing.B) {
		benchmarkE2E(b, false, 0)
	})
	b.Run("ClassicMetrics/MultiSeries", func(b *testing.B) {
		benchmarkE2E(b, false, 100)
	})
	b.Run("NativeHistogram", func(b *testing.B) {
		benchmarkE2E(b, true, 0)
	})
}

func benchmarkE2E(b *testing.B, useNativeHistograms bool, numFamilies int) {
	var labelSets []labels.Labels
	if numFamilies > 0 {
		labelSets = generateMultiSeriesLabelSets(numSeries, 50, numFamilies)
	} else {
		labelSets = generateLabelSets(numSeries, 50)
	}
	var histograms []*histogram.Histogram
	if useNativeHistograms {
		histograms = generateNativeHistograms(numSeries)
	}
	timestamp := int64(1234567890)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tx := newBenchmarkTransaction(b)
		if useNativeHistograms {
			for j := range labelSets {
				_, err := tx.Append(0, labelSets[j], 0, timestamp, 0, histograms[j], nil, storage.AOptions{})
				assert.NoError(b, err)
			}
		} else {
			for j, ls := range labelSets {
				_, err := tx.Append(0, ls, 0, timestamp, float64(j), nil, nil, storage.AOptions{})
				assert.NoError(b, err)
			}
		}
		err := tx.Commit()
		assert.NoError(b, err)
	}
}

func generateMultiSeriesLabelSets(seriesCount, cardinality, numFamilies int) []labels.Labels {
	result := make([]labels.Labels, seriesCount)

	for i := range seriesCount {
		lbls := labels.NewBuilder(labels.EmptyLabels())
		lbls.Set(model.MetricNameLabel, fmt.Sprintf("metric_%d", i%numFamilies))

		for j := range cardinality {
			lbls.Set(fmt.Sprintf("label_%d", j), fmt.Sprintf("value_%d_%d", i, j))
		}

		result[i] = lbls.Labels()
	}

	return result
}

// newBenchmarkTransaction creates a new transaction configured for benchmarking.
// It uses a no-op consumer and minimal configuration to isolate transaction performance.
func newBenchmarkTransaction(b *testing.B) *transaction {
	b.Helper()

	sink := new(consumertest.MetricsSink)
	settings := receivertest.NewNopSettings(mdata.Type)
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.MustNewID("prometheus"),
		Transport:              "http",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		b.Fatalf("Failed to create ObsReport: %v", err)
	}

	tx := newTransaction(
		benchCtx,
		sink,
		labels.EmptyLabels(), // no external labels
		settings,
		obsrecv,
		false, // trimSuffixes
		false, // useMetadata
	)

	// Set a mock MetricMetadataStore to avoid nil pointer issues
	tx.mc = &mockMetadataStore{}

	return tx
}

// generateLabelSets creates label sets for benchmarking with the specified cardinality.
//
//nolint:unparam
func generateLabelSets(seriesCount, cardinality int) []labels.Labels {
	result := make([]labels.Labels, seriesCount)

	for i := range seriesCount {
		lbls := labels.NewBuilder(labels.EmptyLabels())
		lbls.Set(model.MetricNameLabel, fmt.Sprintf("metric_%d", i))

		for j := range cardinality {
			lbls.Set(fmt.Sprintf("label_%d", j), fmt.Sprintf("value_%d_%d", i, j))
		}

		result[i] = lbls.Labels()
	}

	return result
}

// generateNativeHistograms creates native histogram instances for benchmarking.
// Uses Prometheus's test histogram generator for realistic native histograms
func generateNativeHistograms(count int) []*histogram.Histogram {
	result := make([]*histogram.Histogram, count)

	for i := range count {
		result[i] = tsdbutil.GenerateTestHistogram(int64(i))
	}

	return result
}

// createTargetInfoLabels creates labels for a target_info metric.
func createTargetInfoLabels() labels.Labels {
	return labels.FromMap(map[string]string{
		model.MetricNameLabel: prometheus.TargetInfoMetricName,
		model.JobLabel:        "benchmark",
		model.InstanceLabel:   "localhost:8080",
		"environment":         "test",
		"region":              "us-west-2",
		"cluster":             "benchmark-cluster",
	})
}

// createScopeInfoLabels creates labels for an otel_scope_info metric.
func createScopeInfoLabels() labels.Labels {
	return labels.FromMap(map[string]string{
		model.MetricNameLabel:           prometheus.ScopeInfoMetricName,
		model.JobLabel:                  "benchmark",
		model.InstanceLabel:             "localhost:8080",
		prometheus.ScopeNameLabelKey:    "benchmark.scope",
		prometheus.ScopeVersionLabelKey: "1.0.0",
		"scope_attribute":               "test_value",
	})
}

// mockMetadataStore is a minimal implementation of scrape.MetricMetadataStore for testing
type mockMetadataStore struct{}

func (*mockMetadataStore) ListMetadata() []scrape.MetricMetadata {
	return nil
}

func (*mockMetadataStore) GetMetadata(_ string) (scrape.MetricMetadata, bool) {
	return scrape.MetricMetadata{}, false
}

func (*mockMetadataStore) SizeMetadata() int {
	return 0
}

func (*mockMetadataStore) LengthMetadata() int {
	return 0
}

type benchPayload struct {
	textBytes  []byte
	protoBytes []byte
}

// BenchmarkScrapePayload benchmarks the full CPU and memory usage of scraping and committing
// a 1,000-series metrics payload without network calls using Prometheus Protobuf format
// (representing grouped wire formats such as Protobuf and OpenMetrics 2.0) as well as
// Prometheus text format (text/plain; version=0.0.4) for ungrouped multi-line classic histograms.
func BenchmarkScrapePayload(b *testing.B) {
	b.Run("Counter", func(b *testing.B) {
		// 1,000 counters in Protobuf format
		p := benchPayload{protoBytes: generateProtobufCounterPayload(1000)}
		runScrapePayloadBenchmark(b, p)
	})
	b.Run("Gauge", func(b *testing.B) {
		// 1,000 gauges in Protobuf format
		p := benchPayload{protoBytes: generateProtobufGaugePayload(1000)}
		runScrapePayloadBenchmark(b, p)
	})
	b.Run("ClassicHistogram_Proto", func(b *testing.B) {
		// 100 classic histograms * 18 series (16 buckets + sum + count) = 1,800 series equivalent,
		// already grouped in Protobuf wire format (representative of Protobuf and OpenMetrics 2.0).
		p := benchPayload{protoBytes: generateProtobufClassicHistogramPayload(100)}
		runScrapePayloadBenchmark(b, p)
	})
	b.Run("ClassicHistogram_Text", func(b *testing.B) {
		// 100 multi-line classic histograms * 18 lines (16 buckets + sum + count) = 1,800 series lines
		// in Prometheus text format (text/plain; version=0.0.4).
		p := benchPayload{textBytes: generatePromTextClassicHistogramPayload(100)}
		runScrapePayloadBenchmark(b, p)
	})
	b.Run("Summary", func(b *testing.B) {
		// 200 summaries * 7 series (5 quantiles + sum + count) = 1,400 series equivalent in Protobuf format
		p := benchPayload{protoBytes: generateProtobufSummaryPayload(200)}
		runScrapePayloadBenchmark(b, p)
	})
	b.Run("NativeHistogram", func(b *testing.B) {
		// 1,000 native histograms in Protobuf format
		p := benchPayload{protoBytes: generateProtobufNativeHistogramPayload(1000)}
		runScrapePayloadBenchmark(b, p)
	})
	b.Run("MixedPayload", func(b *testing.B) {
		// 200 Counters + 200 Gauges + 20 ClassicHistograms (200 series) + 40 Summaries (200 series) + 200 NativeHistograms = 1,000 series in Protobuf format
		var protoBuf bytes.Buffer
		protoBuf.Write(generateProtobufCounterPayload(200))
		protoBuf.Write(generateProtobufGaugePayload(200))
		protoBuf.Write(generateProtobufClassicHistogramPayload(20))
		protoBuf.Write(generateProtobufSummaryPayload(40))
		protoBuf.Write(generateProtobufNativeHistogramPayload(200))
		p := benchPayload{
			protoBytes: protoBuf.Bytes(),
		}
		runScrapePayloadBenchmark(b, p)
	})
}

func runScrapePayloadBenchmark(b *testing.B, payload benchPayload) {
	settings := receivertest.NewNopSettings(mdata.Type)
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.MustNewID("prometheus"),
		Transport:              "http",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		b.Fatalf("Failed to create ObsReport: %v", err)
	}
	fallbackExemplarLabels := labels.FromStrings("trace_id", "0102030405060708090a0b0c0d0e0f10", "span_id", "0102030405060708")
	symbolTable := labels.NewSymbolTable()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		metaMap := make(testMetadataStore)
		ctx := scrape.ContextWithMetricMetadataStore(benchCtx, metaMap)
		tx := newTransaction(
			ctx,
			consumertest.NewNop(),
			labels.EmptyLabels(),
			settings,
			obsrecv,
			false,
			true,
		)
		tx.mc = metaMap

		if len(payload.textBytes) > 0 {
			parseAndAppend(b, tx, metaMap, payload.textBytes, promTextType, symbolTable, fallbackExemplarLabels)
		}
		if len(payload.protoBytes) > 0 {
			parseAndAppend(b, tx, metaMap, payload.protoBytes, protoType, symbolTable, fallbackExemplarLabels)
		}
		if err := tx.Commit(); err != nil {
			b.Fatalf("Commit failed: %v", err)
		}
	}
}

func parseAndAppend(
	b *testing.B,
	tx *transaction,
	metaMap testMetadataStore,
	data []byte,
	contentType string,
	st *labels.SymbolTable,
	fallbackExemplarLabels labels.Labels,
) {
	p, err := textparse.New(data, contentType, st, textparse.ParserOptions{
		ConvertClassicHistogramsToNHCB: false,
		OpenMetricsSkipSTSeries:        false,
	})
	if err != nil && p == nil {
		b.Fatalf("Failed to create parser: %v", err)
	}
	var lset labels.Labels
	var currMFName string
	var currMeta metadata.Metadata
	exs := make([]exemplar.Exemplar, 0, 8)
	var ex exemplar.Exemplar

	for {
		et, err := p.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
		switch et {
		case textparse.EntryType:
			mName, mType := p.Type()
			currMFName = string(mName)
			currMeta.Type = mType
			md := scrape.MetricMetadata{
				MetricFamily: currMFName,
				Type:         mType,
				Help:         currMeta.Help,
				Unit:         currMeta.Unit,
			}
			metaMap[currMFName] = md
			switch mType {
			case model.MetricTypeCounter:
				metaMap[currMFName+"_total"] = md
			case model.MetricTypeHistogram:
				metaMap[currMFName+"_bucket"] = md
				metaMap[currMFName+"_sum"] = md
				metaMap[currMFName+"_count"] = md
			case model.MetricTypeSummary:
				metaMap[currMFName+"_sum"] = md
				metaMap[currMFName+"_count"] = md
			}
		case textparse.EntryHelp:
			mName, mHelp := p.Help()
			currMFName = string(mName)
			currMeta.Help = string(mHelp)
			md := metaMap[currMFName]
			md.MetricFamily = currMFName
			md.Help = currMeta.Help
			metaMap[currMFName] = md
		case textparse.EntryUnit:
			mName, mUnit := p.Unit()
			currMFName = string(mName)
			currMeta.Unit = string(mUnit)
			md := metaMap[currMFName]
			md.MetricFamily = currMFName
			md.Unit = currMeta.Unit
			metaMap[currMFName] = md
		case textparse.EntrySeries:
			_, tsPtr, val := p.Series()
			p.Labels(&lset)
			ts := int64(1700000000000)
			if tsPtr != nil {
				ts = *tsPtr
			}
			exs = exs[:0]
			for p.Exemplar(&ex) {
				exs = append(exs, ex)
			}
			if len(exs) == 0 {
				exs = append(exs, exemplar.Exemplar{
					Labels: fallbackExemplarLabels,
					Value:  val,
					Ts:     ts,
				})
			}
			if _, err := tx.Append(0, lset, 0, ts, val, nil, nil, storage.AOptions{
				MetricFamilyName: currMFName,
				Metadata:         currMeta,
				Exemplars:        exs,
			}); err != nil {
				b.Fatalf("Append error: %v", err)
			}
		case textparse.EntryHistogram:
			_, tsPtr, h, fh := p.Histogram()
			p.Labels(&lset)
			ts := int64(1700000000000)
			if tsPtr != nil {
				ts = *tsPtr
			}
			exs = exs[:0]
			for p.Exemplar(&ex) {
				exs = append(exs, ex)
			}
			if len(exs) == 0 {
				exs = append(exs, exemplar.Exemplar{
					Labels: fallbackExemplarLabels,
					Value:  1.0,
					Ts:     ts,
				})
			}
			if _, err := tx.Append(0, lset, 0, ts, 0, h, fh, storage.AOptions{
				MetricFamilyName: currMFName,
				Metadata:         currMeta,
				Exemplars:        exs,
			}); err != nil {
				b.Fatalf("Append histogram error: %v", err)
			}
		}
	}
}

func writeDelimitedProto(buf *bytes.Buffer, mf *dto.MetricFamily) {
	data, err := mf.Marshal()
	if err != nil {
		panic(err)
	}
	var varintBuf [binary.MaxVarintLen32]byte
	n := binary.PutUvarint(varintBuf[:], uint64(len(data)))
	buf.Write(varintBuf[:n])
	buf.Write(data)
}

func commonProtoLabels(f, i int) []dto.LabelPair {
	return []dto.LabelPair{
		{Name: "job", Value: "benchmark"},
		{Name: "instance", Value: "localhost:8080"},
		{Name: "service", Value: fmt.Sprintf("svc_%d", f)},
		{Name: "env", Value: "prod"},
		{Name: "region", Value: "us-east-1"},
		{Name: "pod", Value: fmt.Sprintf("pod_%d", i)},
		{Name: "endpoint", Value: "/api/v1/items"},
		{Name: "method", Value: "GET"},
		{Name: "status", Value: "200"},
	}
}

func protoExemplar(val float64, tsProto *types.Timestamp) *dto.Exemplar {
	return &dto.Exemplar{
		Value:     val,
		Timestamp: tsProto,
		Label: []dto.LabelPair{
			{Name: "trace_id", Value: "0102030405060708090a0b0c0d0e0f10"},
			{Name: "span_id", Value: "0102030405060708"},
		},
	}
}

func generateProtobufCounterPayload(count int) []byte {
	var buf bytes.Buffer
	numFamilies := min(10, count)
	perFamily := count / numFamilies
	tsProto := &types.Timestamp{Seconds: 1700000000, Nanos: 0}
	for f := range numFamilies {
		mfName := fmt.Sprintf("bench_counter_%d_total", f)
		mf := &dto.MetricFamily{
			Name:   mfName,
			Help:   fmt.Sprintf("Benchmark counter metric family %d", f),
			Type:   dto.MetricType_COUNTER,
			Metric: make([]dto.Metric, 0, perFamily),
		}
		for i := range perFamily {
			mf.Metric = append(mf.Metric, dto.Metric{
				Label:       commonProtoLabels(f, i),
				TimestampMs: 1700000000000,
				Counter: &dto.Counter{
					Value:    float64(i + 1),
					Exemplar: protoExemplar(1.0, tsProto),
				},
			})
		}
		writeDelimitedProto(&buf, mf)
	}
	return buf.Bytes()
}

func generateProtobufGaugePayload(count int) []byte {
	var buf bytes.Buffer
	numFamilies := min(10, count)
	perFamily := count / numFamilies
	for f := range numFamilies {
		mfName := fmt.Sprintf("bench_gauge_%d", f)
		mf := &dto.MetricFamily{
			Name:   mfName,
			Help:   fmt.Sprintf("Benchmark gauge metric family %d", f),
			Type:   dto.MetricType_GAUGE,
			Metric: make([]dto.Metric, 0, perFamily),
		}
		for i := range perFamily {
			mf.Metric = append(mf.Metric, dto.Metric{
				Label:       commonProtoLabels(f, i),
				TimestampMs: 1700000000000,
				Gauge: &dto.Gauge{
					Value: float64(i) + 0.5,
				},
			})
		}
		writeDelimitedProto(&buf, mf)
	}
	return buf.Bytes()
}

func generateProtobufClassicHistogramPayload(numHistograms int) []byte {
	var buf bytes.Buffer
	numFamilies := min(10, numHistograms)
	perFamily := numHistograms / numFamilies
	// Default OpenTelemetry ExplicitBucketHistogram boundaries (15 bounds) + +Inf = 16 buckets.
	bounds := []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000, math.Inf(1)}
	tsProto := &types.Timestamp{Seconds: 1700000000, Nanos: 0}
	for f := range numFamilies {
		mfName := fmt.Sprintf("bench_classic_hist_%d", f)
		mf := &dto.MetricFamily{
			Name:   mfName,
			Help:   fmt.Sprintf("Benchmark classic histogram family %d", f),
			Type:   dto.MetricType_HISTOGRAM,
			Metric: make([]dto.Metric, 0, perFamily),
		}
		for i := range perFamily {
			buckets := make([]dto.Bucket, len(bounds))
			for bIdx, ub := range bounds {
				cumCount := uint64((bIdx + 1) * 10)
				buckets[bIdx] = dto.Bucket{
					CumulativeCount: cumCount,
					UpperBound:      ub,
					Exemplar:        protoExemplar(0.042, tsProto),
				}
			}
			mf.Metric = append(mf.Metric, dto.Metric{
				Label:       commonProtoLabels(f, i),
				TimestampMs: 1700000000000,
				Histogram: &dto.Histogram{
					SampleCount: uint64(len(bounds) * 10),
					SampleSum:   45.67,
					Bucket:      buckets,
				},
			})
		}
		writeDelimitedProto(&buf, mf)
	}
	return buf.Bytes()
}

func generatePromTextClassicHistogramPayload(numHistograms int) []byte {
	var buf bytes.Buffer
	numFamilies := min(10, numHistograms)
	perFamily := numHistograms / numFamilies
	// Default OpenTelemetry ExplicitBucketHistogram boundaries (15 bounds) + +Inf = 16 buckets.
	bounds := []string{"0", "5", "10", "25", "50", "75", "100", "250", "500", "750", "1000", "2500", "5000", "7500", "10000", "+Inf"}
	for f := range numFamilies {
		mf := fmt.Sprintf("bench_classic_hist_%d", f)
		fmt.Fprintf(&buf, "# TYPE %s histogram\n", mf)
		fmt.Fprintf(&buf, "# HELP %s Benchmark classic histogram family %d\n", mf, f)
		for i := range perFamily {
			baseLabels := fmt.Sprintf("job=\"benchmark\",instance=\"localhost:8080\",service=\"svc_%d\",env=\"prod\",region=\"us-east-1\",pod=\"pod_%d\",endpoint=\"/api/v1/items\",method=\"GET\",status=\"200\"", f, i)
			for bIdx, le := range bounds {
				cumCount := (bIdx + 1) * 10
				fmt.Fprintf(&buf, "%s_bucket{%s,le=\"%s\"} %d 1700000000000\n", mf, baseLabels, le, cumCount)
			}
			fmt.Fprintf(&buf, "%s_sum{%s} 45.67 1700000000000\n", mf, baseLabels)
			fmt.Fprintf(&buf, "%s_count{%s} %d 1700000000000\n", mf, baseLabels, len(bounds)*10)
		}
	}
	return buf.Bytes()
}

func generateProtobufSummaryPayload(numSummaries int) []byte {
	var buf bytes.Buffer
	numFamilies := min(10, numSummaries)
	perFamily := numSummaries / numFamilies
	// 5 quantiles matching Prometheus's default go_gc_duration_seconds summary (0, 0.25, 0.5, 0.75, 1).
	quantiles := []dto.Quantile{
		{Quantile: 0.0, Value: 0.01},
		{Quantile: 0.25, Value: 0.05},
		{Quantile: 0.5, Value: 0.12},
		{Quantile: 0.75, Value: 0.45},
		{Quantile: 1.0, Value: 0.89},
	}
	for f := range numFamilies {
		mfName := fmt.Sprintf("bench_summary_%d", f)
		mf := &dto.MetricFamily{
			Name:   mfName,
			Help:   fmt.Sprintf("Benchmark summary family %d", f),
			Type:   dto.MetricType_SUMMARY,
			Metric: make([]dto.Metric, 0, perFamily),
		}
		for i := range perFamily {
			mf.Metric = append(mf.Metric, dto.Metric{
				Label:       commonProtoLabels(f, i),
				TimestampMs: 1700000000000,
				Summary: &dto.Summary{
					SampleCount: 50,
					SampleSum:   23.45,
					Quantile:    quantiles,
				},
			})
		}
		writeDelimitedProto(&buf, mf)
	}
	return buf.Bytes()
}

func generateProtobufNativeHistogramPayload(count int) []byte {
	var buf bytes.Buffer
	numFamilies := min(10, count)
	perFamily := count / numFamilies
	tsProto := &types.Timestamp{Seconds: 1700000000, Nanos: 0}
	for f := range numFamilies {
		mfName := fmt.Sprintf("bench_native_hist_%d", f)
		mf := &dto.MetricFamily{
			Name:   mfName,
			Help:   fmt.Sprintf("Benchmark native histogram family %d", f),
			Type:   dto.MetricType_HISTOGRAM,
			Metric: make([]dto.Metric, 0, perFamily),
		}
		for i := range perFamily {
			mf.Metric = append(mf.Metric, dto.Metric{
				Label:       commonProtoLabels(f, i),
				TimestampMs: 1700000000000,
				Histogram: &dto.Histogram{
					SampleCount:   66,
					SampleSum:     1004.78,
					Schema:        3,
					ZeroThreshold: 0.001,
					ZeroCount:     2,
					PositiveSpan: []dto.BucketSpan{
						{Offset: 0, Length: 4},
					},
					PositiveDelta: []int64{10, 5, -3, 2},
					Exemplars: []*dto.Exemplar{
						protoExemplar(0.42, tsProto),
					},
				},
			})
		}
		writeDelimitedProto(&buf, mf)
	}
	return buf.Bytes()
}
