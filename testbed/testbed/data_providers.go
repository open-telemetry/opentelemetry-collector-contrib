// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

import (
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/idutils"
)

// DataProvider defines the interface for generators of test data used to drive various end-to-end tests.
type DataProvider interface {
	// SetLoadGeneratorCounters supplies pointers to LoadGenerator counters.
	// The data provider implementation should increment these as it generates data.
	SetLoadGeneratorCounters(dataItemsGenerated *atomic.Uint64)
	// GenerateTraces returns an internal Traces instance with an OTLP ResourceSpans slice populated with test data.
	GenerateTraces() (ptrace.Traces, bool)
	// GenerateMetrics returns an internal MetricData instance with an OTLP ResourceMetrics slice of test data.
	GenerateMetrics() (pmetric.Metrics, bool)
	// GenerateLogs returns the internal plog.Logs format
	GenerateLogs() (plog.Logs, bool)
}

// perfTestDataProvider in an implementation of the DataProvider for use in performance tests.
// Tracing IDs are based on the incremented batch and data items counters.
type perfTestDataProvider struct {
	options            LoadOptions
	traceIDSequence    atomic.Uint64
	dataItemsGenerated *atomic.Uint64
}

// NewPerfTestDataProvider creates an instance of perfTestDataProvider which generates test data based on the sizes
// specified in the supplied LoadOptions.
func NewPerfTestDataProvider(options LoadOptions) DataProvider {
	return &perfTestDataProvider{
		options: options,
	}
}

func (dp *perfTestDataProvider) SetLoadGeneratorCounters(dataItemsGenerated *atomic.Uint64) {
	dp.dataItemsGenerated = dataItemsGenerated
}

func (dp *perfTestDataProvider) GenerateTraces() (ptrace.Traces, bool) {
	traceData := ptrace.NewTraces()
	spans := traceData.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	spans.EnsureCapacity(dp.options.ItemsPerBatch)

	traceID := dp.traceIDSequence.Add(1)
	for i := 0; i < dp.options.ItemsPerBatch; i++ {

		startTime := time.Now()
		endTime := startTime.Add(time.Millisecond)

		spanID := dp.dataItemsGenerated.Add(1)

		span := spans.AppendEmpty()

		// Create a span.
		span.SetTraceID(idutils.UInt64ToTraceID(0, traceID))
		span.SetSpanID(idutils.UInt64ToSpanID(spanID))
		span.SetName("load-generator-span")
		span.SetKind(ptrace.SpanKindClient)
		attrs := span.Attributes()
		attrs.PutInt("load_generator.span_seq_num", int64(spanID))
		attrs.PutInt("load_generator.trace_seq_num", int64(traceID))
		// Additional attributes.
		for k, v := range dp.options.Attributes {
			attrs.PutStr(k, v)
		}
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))
	}
	return traceData, false
}

func (dp *perfTestDataProvider) GenerateMetrics() (pmetric.Metrics, bool) {
	// Generate 7 data points per metric.
	const dataPointsPerMetric = 7

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	if dp.options.Attributes != nil {
		attrs := rm.Resource().Attributes()
		attrs.EnsureCapacity(len(dp.options.Attributes))
		for k, v := range dp.options.Attributes {
			attrs.PutStr(k, v)
		}
	}
	metrics := rm.ScopeMetrics().AppendEmpty().Metrics()
	metrics.EnsureCapacity(dp.options.ItemsPerBatch)

	for i := 0; i < dp.options.ItemsPerBatch; i++ {
		metric := metrics.AppendEmpty()
		metric.SetDescription("Load Generator Counter #" + strconv.Itoa(i))
		metric.SetUnit("1")
		dps := metric.SetEmptyGauge().DataPoints()
		batchIndex := dp.traceIDSequence.Add(1)
		// Generate data points for the metric.
		dps.EnsureCapacity(dataPointsPerMetric)
		for j := 0; j < dataPointsPerMetric; j++ {
			dataPoint := dps.AppendEmpty()
			dataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			value := dp.dataItemsGenerated.Add(1)
			dataPoint.SetIntValue(int64(value))
			dataPoint.Attributes().PutStr("item_index", "item_"+strconv.Itoa(j))
			dataPoint.Attributes().PutStr("batch_index", "batch_"+strconv.Itoa(int(batchIndex)))
		}
	}
	return md, false
}

func (dp *perfTestDataProvider) GenerateLogs() (plog.Logs, bool) {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	if dp.options.Attributes != nil {
		attrs := rl.Resource().Attributes()
		attrs.EnsureCapacity(len(dp.options.Attributes))
		for k, v := range dp.options.Attributes {
			attrs.PutStr(k, v)
		}
	}
	logRecords := rl.ScopeLogs().AppendEmpty().LogRecords()
	logRecords.EnsureCapacity(dp.options.ItemsPerBatch)

	now := pcommon.NewTimestampFromTime(time.Now())

	batchIndex := dp.traceIDSequence.Add(1)

	for i := 0; i < dp.options.ItemsPerBatch; i++ {
		itemIndex := dp.dataItemsGenerated.Add(1)
		record := logRecords.AppendEmpty()
		record.SetSeverityNumber(plog.SeverityNumberInfo3)
		record.SetSeverityText("INFO3")
		record.Body().SetStr("Load Generator Counter #" + strconv.Itoa(i))
		record.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
		record.SetTimestamp(now)

		attrs := record.Attributes()
		attrs.PutStr("batch_index", "batch_"+strconv.Itoa(int(batchIndex)))
		attrs.PutStr("item_index", "item_"+strconv.Itoa(int(itemIndex)))
		attrs.PutStr("a", "test")
		attrs.PutDouble("b", 5.0)
		attrs.PutInt("c", 3)
		attrs.PutBool("d", true)
	}
	return logs, false
}

// goldenDataProvider is an implementation of DataProvider for use in correctness tests.
// Provided data from the "Golden" dataset generated using pairwise combinatorial testing techniques.
type goldenDataProvider struct {
	tracePairsFile     string
	spanPairsFile      string
	dataItemsGenerated *atomic.Uint64

	tracesGenerated []ptrace.Traces
	tracesIndex     int

	metricPairsFile  string
	metricsGenerated []pmetric.Metrics
	metricsIndex     int
}

// NewGoldenDataProvider creates a new instance of goldenDataProvider which generates test data based
// on the pairwise combinations specified in the tracePairsFile and spanPairsFile input variables.
func NewGoldenDataProvider(tracePairsFile string, spanPairsFile string, metricPairsFile string) DataProvider {
	return &goldenDataProvider{
		tracePairsFile:  tracePairsFile,
		spanPairsFile:   spanPairsFile,
		metricPairsFile: metricPairsFile,
	}
}

func (dp *goldenDataProvider) SetLoadGeneratorCounters(dataItemsGenerated *atomic.Uint64) {
	dp.dataItemsGenerated = dataItemsGenerated
}

func (dp *goldenDataProvider) GenerateTraces() (ptrace.Traces, bool) {
	if dp.tracesGenerated == nil {
		var err error
		dp.tracesGenerated, err = goldendataset.GenerateTraces(dp.tracePairsFile, dp.spanPairsFile)
		if err != nil {
			log.Printf("cannot generate traces: %s", err)
			dp.tracesGenerated = nil
		}
	}
	if dp.tracesIndex >= len(dp.tracesGenerated) {
		return ptrace.NewTraces(), true
	}
	td := dp.tracesGenerated[dp.tracesIndex]
	dp.tracesIndex++
	dp.dataItemsGenerated.Add(uint64(td.SpanCount()))
	return td, false
}

func (dp *goldenDataProvider) GenerateMetrics() (pmetric.Metrics, bool) {
	if dp.metricsGenerated == nil {
		var err error
		dp.metricsGenerated, err = goldendataset.GenerateMetrics(dp.metricPairsFile)
		if err != nil {
			log.Printf("cannot generate metrics: %s", err)
		}
	}
	if dp.metricsIndex == len(dp.metricsGenerated) {
		return pmetric.Metrics{}, true
	}
	pdm := dp.metricsGenerated[dp.metricsIndex]
	dp.metricsIndex++
	dp.dataItemsGenerated.Add(uint64(pdm.DataPointCount()))
	return pdm, false
}

func (dp *goldenDataProvider) GenerateLogs() (plog.Logs, bool) {
	return plog.NewLogs(), true
}

// FileDataProvider in an implementation of the DataProvider for use in performance tests.
// The data to send is loaded from a file. The file should contain one JSON-encoded
// Export*ServiceRequest Protobuf message. The file can be recorded using the "file"
// exporter (note: "file" exporter writes one JSON message per line, FileDataProvider
// expects just a single JSON message in the entire file).
type FileDataProvider struct {
	dataItemsGenerated *atomic.Uint64
	logs               plog.Logs
	metrics            pmetric.Metrics
	traces             ptrace.Traces
	ItemsPerBatch      int
}

// NewFileDataProvider creates an instance of FileDataProvider which generates test data
// loaded from a file.
func NewFileDataProvider(filePath string, dataType component.DataType) (*FileDataProvider, error) {
	dp := &FileDataProvider{}
	var err error
	// Load the message from the file and count the data points.
	switch dataType {
	case component.DataTypeTraces:
		if dp.traces, err = golden.ReadTraces(filePath); err != nil {
			return nil, err
		}
		dp.ItemsPerBatch = dp.traces.SpanCount()
	case component.DataTypeMetrics:
		if dp.metrics, err = golden.ReadMetrics(filePath); err != nil {
			return nil, err
		}
		dp.ItemsPerBatch = dp.metrics.DataPointCount()
	case component.DataTypeLogs:
		if dp.logs, err = golden.ReadLogs(filePath); err != nil {
			return nil, err
		}
		dp.ItemsPerBatch = dp.logs.LogRecordCount()
	}

	return dp, nil
}

func (dp *FileDataProvider) SetLoadGeneratorCounters(dataItemsGenerated *atomic.Uint64) {
	dp.dataItemsGenerated = dataItemsGenerated
}

func (dp *FileDataProvider) GenerateTraces() (ptrace.Traces, bool) {
	dp.dataItemsGenerated.Add(uint64(dp.ItemsPerBatch))
	return dp.traces, false
}

func (dp *FileDataProvider) GenerateMetrics() (pmetric.Metrics, bool) {
	dp.dataItemsGenerated.Add(uint64(dp.ItemsPerBatch))
	return dp.metrics, false
}

func (dp *FileDataProvider) GenerateLogs() (plog.Logs, bool) {
	dp.dataItemsGenerated.Add(uint64(dp.ItemsPerBatch))
	return dp.logs, false
}
