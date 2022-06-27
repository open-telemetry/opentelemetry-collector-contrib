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
	"fmt"
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
	c := Config{ClusterName: "https://CLUSTER.kusto.windows.net",
		ClientId:       "unknown",
		ClientSecret:   "unknown",
		TenantId:       "unknown",
		Database:       "not-configured",
		RawMetricTable: "not-configured",
		RawLogTable:    "RawLogs",
	}
	texp, err := newExporter(&c, logger, metricstype)
	assert.Error(t, err)
	assert.Nil(t, texp)
}

func TestMetricsDataPusherStreaming(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoclient := kusto.NewMockClient()
	ingestoptions := make([]ingest.FileOption, 2)
	ingestoptions[0] = ingest.FileFormat(ingest.JSON)
	ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower("RawMetrics")), ingest.JSON)
	managedstreamingingest, _ := ingest.NewManaged(kustoclient, "testDB", "RawMetrics")

	adxdataproducer := &adxDataProducer{
		client:        kustoclient,
		managedingest: managedstreamingingest,
		ingestoptions: ingestoptions,
		logger:        logger,
	}
	assert.NotNil(t, adxdataproducer)
	err := adxdataproducer.metricsDataPusher(context.Background(), createMetricsData(10))
	assert.NotNil(t, err)
}

func TestMetricsDataPusherQueued(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoclient := kusto.NewMockClient()
	ingestoptions := make([]ingest.FileOption, 2)
	ingestoptions[0] = ingest.FileFormat(ingest.JSON)
	ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower("RawMetrics")), ingest.JSON)
	queuedingest, _ := ingest.New(kustoclient, "testDB", "RawMetrics")

	adxdataproducer := &adxDataProducer{
		client:        kustoclient,
		queuedingest:  queuedingest,
		ingestoptions: ingestoptions,
		logger:        logger,
	}
	assert.NotNil(t, adxdataproducer)
	err := adxdataproducer.metricsDataPusher(context.Background(), createMetricsData(10))
	assert.NotNil(t, err)
	//stmt := kusto.Stmt{"RawMetrics | take 10"}
	//kustoclient.Query(context.Background(), "testDB", stmt)
}

func TestLogsDataPusher(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoclient := kusto.NewMockClient()
	ingestoptions := make([]ingest.FileOption, 2)
	ingestoptions[0] = ingest.FileFormat(ingest.JSON)
	ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower("RawLogs")), ingest.JSON)
	managedstreamingingest, _ := ingest.NewManaged(kustoclient, "testDB", "RawLogs")

	adxdataproducer := &adxDataProducer{
		client:        kustoclient,
		managedingest: managedstreamingingest,
		ingestoptions: ingestoptions,
		logger:        logger,
	}
	assert.NotNil(t, adxdataproducer)
	err := adxdataproducer.logsDataPusher(context.Background(), createLogsData())
	assert.NotNil(t, err)
}

func TestTracesDataPusher(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoclient := kusto.NewMockClient()
	ingestoptions := make([]ingest.FileOption, 2)
	ingestoptions[0] = ingest.FileFormat(ingest.JSON)
	ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower("RawLogs")), ingest.JSON)
	managedstreamingingest, _ := ingest.NewManaged(kustoclient, "testDB", "RawLogs")

	adxdataproducer := &adxDataProducer{
		client:        kustoclient,
		managedingest: managedstreamingingest,
		ingestoptions: ingestoptions,
		logger:        logger,
	}
	assert.NotNil(t, adxdataproducer)
	err := adxdataproducer.tracesDataPusher(context.Background(), createTracesData())
	assert.NotNil(t, err)
}

func TestClose(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoclient := kusto.NewMockClient()
	ingestoptions := make([]ingest.FileOption, 2)
	ingestoptions[0] = ingest.FileFormat(ingest.MultiJSON)
	ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower("RawMetrics")), ingest.MultiJSON)
	managedstreamingingest, _ := ingest.NewManaged(kustoclient, "testDB", "RawMetrics")

	adxdataproducer := &adxDataProducer{
		client:        kustoclient,
		managedingest: managedstreamingingest,
		ingestoptions: ingestoptions,
		logger:        logger,
	}
	err := adxdataproducer.Close(context.Background())
	assert.Nil(t, err)
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
