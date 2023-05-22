// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"context"
	"io"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
)

func TestNewExporter(t *testing.T) {
	logger := zaptest.NewLogger(t)
	c := Config{ClusterURI: "https://CLUSTER.kusto.windows.net",
		ApplicationID:      "unknown",
		ApplicationKey:     "unknown",
		TenantID:           "unknown",
		Database:           "not-configured",
		MetricTable:        "OTELMetrics",
		LogTable:           "OTELLogs",
		TraceTable:         "OTELTraces",
		MetricTableMapping: "otelmetrics_mapping",
		LogTableMapping:    "otellogs_mapping",
		TraceTableMapping:  "oteltraces_mapping",
	}
	texp, err := newExporter(&c, logger, metricsType, component.NewDefaultBuildInfo().Version)
	assert.NoError(t, err)
	assert.NotNil(t, texp)
	texp, err = newExporter(&c, logger, logsType, component.NewDefaultBuildInfo().Version)
	assert.NoError(t, err)
	assert.NotNil(t, texp)
	texp, err = newExporter(&c, logger, tracesType, component.NewDefaultBuildInfo().Version)
	assert.NoError(t, err)
	assert.NotNil(t, texp)
	texp, err = newExporter(&c, logger, 5, component.NewDefaultBuildInfo().Version)
	assert.Error(t, err)
	assert.Nil(t, texp)
}

func TestMetricsDataPusherStreaming(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoClient := kusto.NewMockClient()
	var ingestOptions []ingest.FileOption
	ingestOptions = append(ingestOptions, ingest.FileFormat(ingest.JSON))
	managedStreamingIngest, _ := ingest.NewManaged(kustoClient, "testDB", "OTELMetrics")

	adxDataProducer := &adxDataProducer{
		client:        kustoClient,
		ingestor:      managedStreamingIngest,
		ingestOptions: ingestOptions,
		logger:        logger,
	}
	assert.NotNil(t, adxDataProducer)
	err := adxDataProducer.metricsDataPusher(context.Background(), createMetricsData(10))
	assert.NotNil(t, err)
}

func TestMetricsDataPusherQueued(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoClient := kusto.NewMockClient()
	var ingestOptions []ingest.FileOption
	ingestOptions = append(ingestOptions, ingest.FileFormat(ingest.JSON))
	queuedingest, _ := ingest.New(kustoClient, "testDB", "OTELMetrics")

	adxDataProducer := &adxDataProducer{
		client:        kustoClient,
		ingestor:      queuedingest,
		ingestOptions: ingestOptions,
		logger:        logger,
	}
	assert.NotNil(t, adxDataProducer)
	err := adxDataProducer.metricsDataPusher(context.Background(), createMetricsData(10))
	assert.NotNil(t, err)
}

func TestLogsDataPusher(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoClient := kusto.NewMockClient()
	var ingestOptions []ingest.FileOption
	ingestOptions = append(ingestOptions, ingest.FileFormat(ingest.JSON))
	managedStreamingIngest, _ := ingest.NewManaged(kustoClient, "testDB", "OTELLogs")

	adxDataProducer := &adxDataProducer{
		client:        kustoClient,
		ingestor:      managedStreamingIngest,
		ingestOptions: ingestOptions,
		logger:        logger,
	}
	assert.NotNil(t, adxDataProducer)
	err := adxDataProducer.logsDataPusher(context.Background(), createLogsData())
	assert.NotNil(t, err)
}

func TestTracesDataPusher(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoClient := kusto.NewMockClient()
	var ingestOptions []ingest.FileOption
	ingestOptions = append(ingestOptions, ingest.FileFormat(ingest.JSON))
	managedStreamingIngest, _ := ingest.NewManaged(kustoClient, "testDB", "OTELLogs")

	adxDataProducer := &adxDataProducer{
		client:        kustoClient,
		ingestor:      managedStreamingIngest,
		ingestOptions: ingestOptions,
		logger:        logger,
	}
	assert.NotNil(t, adxDataProducer)
	err := adxDataProducer.tracesDataPusher(context.Background(), createTracesData())
	assert.NotNil(t, err)
}

func TestClose(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoClient := kusto.NewMockClient()
	var ingestOptions []ingest.FileOption
	ingestOptions = append(ingestOptions, ingest.FileFormat(ingest.JSON))
	managedStreamingIngest, _ := ingest.NewManaged(kustoClient, "testDB", "OTELMetrics")

	adxDataProducer := &adxDataProducer{
		client:        kustoClient,
		ingestor:      managedStreamingIngest,
		ingestOptions: ingestOptions,
		logger:        logger,
	}
	err := adxDataProducer.Close(context.Background())
	assert.Nil(t, err)
}

func TestIngestedDataRecordCount(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoClient := kusto.NewMockClient()
	ingestOptions := make([]ingest.FileOption, 2)
	ingestOptions[0] = ingest.FileFormat(ingest.JSON)
	ingestor := &mockingestor{}

	adxDataProducer := &adxDataProducer{
		client:        kustoClient,
		ingestor:      ingestor,
		ingestOptions: ingestOptions,
		logger:        logger,
	}
	source := rand.NewSource(time.Now().UTC().UnixNano())
	genRand := rand.New(source)
	recordstoingest := genRand.Intn(20)
	err := adxDataProducer.metricsDataPusher(context.Background(), createMetricsData(recordstoingest))
	ingestedrecordsactual := ingestor.Records()
	assert.Equal(t, recordstoingest, len(ingestedrecordsactual), "Number of metrics created should match number of records ingested")
	assert.Nil(t, err)
}

type mockingestor struct {
	records []string
}

func (m *mockingestor) FromReader(ctx context.Context, reader io.Reader, options ...ingest.FileOption) (*ingest.Result, error) {
	bufbytes, _ := io.ReadAll(reader)
	metricjson := string(bufbytes)
	m.SetRecords(strings.Split(metricjson, "\n"))
	return &ingest.Result{}, nil
}

func (m *mockingestor) FromFile(ctx context.Context, fPath string, options ...ingest.FileOption) (*ingest.Result, error) {
	return &ingest.Result{}, nil
}

func (m *mockingestor) SetRecords(records []string) {
	m.records = records
}

// Name receives a copy of Foo since it doesn't need to modify it.
func (m *mockingestor) Records() []string {
	return m.records
}

func (m *mockingestor) Close() error {
	return nil
}

func createMetricsData(numberOfDataPoints int) pmetric.Metrics {
	doubleVal := 1234.5678
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("k0", "v0")
	for i := 0; i < numberOfDataPoints; i++ {
		tsUnix := time.Unix(time.Now().Unix(), time.Now().UnixNano())
		ilm := rm.ScopeMetrics().AppendEmpty()
		metric := ilm.Metrics().AppendEmpty()
		metric.SetName("gauge_double_with_dims")
		metric.SetEmptyGauge()
		doublePt := metric.Gauge().DataPoints().AppendEmpty()
		doublePt.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
		doublePt.SetDoubleValue(doubleVal)
	}
	return metrics
}

func createLogsData() plog.Logs {
	spanID := [8]byte{0, 0, 0, 0, 0, 0, 0, 50}
	traceID := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100}

	logs := plog.NewLogs()
	rm := logs.ResourceLogs().AppendEmpty()
	rm.Resource().Attributes().PutStr("k0", "v0")
	ism := rm.ScopeLogs().AppendEmpty()
	ism.Scope().SetName("scopename")
	ism.Scope().SetVersion("1.0")
	log := ism.LogRecords().AppendEmpty()
	log.Body().SetStr("mylogsample")
	log.Attributes().PutStr("test", "value")
	log.SetTimestamp(ts)
	log.SetSpanID(pcommon.SpanID(spanID))
	log.SetTraceID(pcommon.TraceID(traceID))
	log.SetSeverityNumber(plog.SeverityNumberDebug)
	log.SetSeverityText("DEBUG")
	return logs

}

func createTracesData() ptrace.Traces {
	spanID := [8]byte{0, 0, 0, 0, 0, 0, 0, 50}
	traceID := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100}

	traces := ptrace.NewTraces()
	rm := traces.ResourceSpans().AppendEmpty()
	rm.Resource().Attributes().PutStr("host", "test")
	ism := rm.ScopeSpans().AppendEmpty()
	ism.Scope().SetName("Scopename")
	ism.Scope().SetVersion("1.0")
	span := ism.Spans().AppendEmpty()
	span.SetName("spanname")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(ts)
	span.SetEndTimestamp(ts)
	span.SetSpanID(pcommon.SpanID(spanID))
	span.SetTraceID(pcommon.TraceID(traceID))
	span.TraceState().FromRaw("")
	span.Attributes().PutStr("key", "val")
	return traces
}
