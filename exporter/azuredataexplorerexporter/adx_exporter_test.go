// Copyright OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuredataexplorerexporter

import (
	"context"
	"io"
	"io/ioutil"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
)

func TestNewExporter(t *testing.T) {
	logger := zaptest.NewLogger(t)
	c := Config{ClusterUri: "https://CLUSTER.kusto.windows.net",
		ApplicationId:          "unknown",
		ApplicationKey:         "unknown",
		TenantId:               "unknown",
		Database:               "not-configured",
		OTELMetricTable:        "OTELMetrics",
		OTELLogTable:           "OTELLogs",
		OTELTraceTable:         "OTELTraces",
		OTELMetricTableMapping: "otelmetrics_mapping",
		OTELLogTableMapping:    "otellogs_mapping",
		OTELTraceTableMapping:  "oteltraces_mapping",
	}
	texp, err := newExporter(&c, logger, metricsType)
	assert.Error(t, err)
	assert.Nil(t, texp)
	texp, err = newExporter(&c, logger, logsType)
	assert.Error(t, err)
	assert.Nil(t, texp)
	texp, err = newExporter(&c, logger, tracesType)
	assert.Error(t, err)
	assert.Nil(t, texp)
	texp, err = newExporter(&c, logger, 5)
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
	rand.Seed(time.Now().UTC().UnixNano())
	recordstoingest := rand.Intn(20)
	err := adxDataProducer.metricsDataPusher(context.Background(), createMetricsData(recordstoingest))
	ingestedrecordsactual := ingestor.Records()
	assert.Equal(t, recordstoingest, len(ingestedrecordsactual), "Number of metrics created should match number of records ingested")
	assert.Nil(t, err)
}

type mockingestor struct {
	records []string
}

func (m *mockingestor) FromReader(ctx context.Context, reader io.Reader, options ...ingest.FileOption) (*ingest.Result, error) {
	bufbytes, _ := ioutil.ReadAll(reader)
	metricjson := string(bufbytes)
	m.SetRecords(strings.Split(metricjson, "\n"))
	return &ingest.Result{}, nil
}

func (m *mockingestor) FromFile(ctx context.Context, fPath string, options ...ingest.FileOption) (*ingest.Result, error) {
	return &ingest.Result{}, nil
}

func (f *mockingestor) SetRecords(records []string) {
	f.records = records
}

// Name receives a copy of Foo since it doesn't need to modify it.
func (f *mockingestor) Records() []string {
	return f.records
}

func (m *mockingestor) Close() error {
	return nil
}

func createMetricsData(numberOfDataPoints int) pmetric.Metrics {
	doubleVal := 1234.5678
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().InsertString("k0", "v0")
	for i := 0; i < numberOfDataPoints; i++ {
		tsUnix := time.Unix(time.Now().Unix(), time.Now().UnixNano())
		ilm := rm.ScopeMetrics().AppendEmpty()
		metric := ilm.Metrics().AppendEmpty()
		metric.SetName("gauge_double_with_dims")
		metric.SetDataType(pmetric.MetricDataTypeGauge)
		doublePt := metric.Gauge().DataPoints().AppendEmpty()
		doublePt.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
		doublePt.SetDoubleVal(doubleVal)
	}
	return metrics
}

func createLogsData() plog.Logs {
	spanId := [8]byte{0, 0, 0, 0, 0, 0, 0, 50}
	traceId := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100}

	logs := plog.NewLogs()
	rm := logs.ResourceLogs().AppendEmpty()
	rm.Resource().Attributes().InsertString("k0", "v0")
	ism := rm.ScopeLogs().AppendEmpty()
	ism.Scope().SetName("scopename")
	ism.Scope().SetVersion("1.0")
	log := ism.LogRecords().AppendEmpty()
	log.Body().SetStringVal("mylogsample")
	log.Attributes().InsertString("test", "value")
	log.SetTimestamp(ts)
	log.SetSpanID(pcommon.NewSpanID(spanId))
	log.SetTraceID(pcommon.NewTraceID(traceId))
	log.SetSeverityNumber(plog.SeverityNumberDEBUG)
	log.SetSeverityText("DEBUG")
	return logs

}

func createTracesData() ptrace.Traces {
	spanId := [8]byte{0, 0, 0, 0, 0, 0, 0, 50}
	traceId := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 100}

	traces := ptrace.NewTraces()
	rm := traces.ResourceSpans().AppendEmpty()
	rm.Resource().Attributes().InsertString("host", "test")
	ism := rm.ScopeSpans().AppendEmpty()
	ism.Scope().SetName("Scopename")
	ism.Scope().SetVersion("1.0")
	span := ism.Spans().AppendEmpty()
	span.SetName("spanname")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(ts)
	span.SetEndTimestamp(ts)
	span.SetSpanID(pcommon.NewSpanID(spanId))
	span.SetTraceID(pcommon.NewTraceID(traceId))
	span.SetTraceState(ptrace.TraceStateEmpty)
	span.Attributes().InsertString("key", "val")
	return traces
}
