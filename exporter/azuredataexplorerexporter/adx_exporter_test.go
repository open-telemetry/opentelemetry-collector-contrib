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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
)

func TestNewExporter_err_version(t *testing.T) {
	logger := zaptest.NewLogger(t)
	c := Config{ClusterName: "https://CLUSTER.kusto.windows.net",
		ClientId:       "unknown",
		ClientSecret:   "unknown",
		TenantId:       "unknown",
		Database:       "not-configured",
		RawMetricTable: "not-configured",
	}
	texp, err := newExporter(&c, logger, metricsType)
	assert.Error(t, err)
	assert.Nil(t, texp)
}

func TestMetricsDataPusher(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoclient := kusto.NewMockClient()
	ingestoptions := make([]ingest.FileOption, 2)
	ingestoptions[0] = ingest.FileFormat(ingest.MultiJSON)
	ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower("RawMetrics")), ingest.MultiJSON)
	managedstreamingingest, _ := ingest.NewManaged(kustoclient, "testDB", "RawMetrics")

	adxMetricsProducer := &adxDataProducer{
		client:        kustoclient,
		managedingest: managedstreamingingest,
		ingestoptions: ingestoptions,
		logger:        logger,
	}
	assert.NotNil(t, adxMetricsProducer)
	err := adxMetricsProducer.metricsDataPusher(context.Background(), createMetricsData(10))
	assert.NotNil(t, err)
	//stmt := kusto.Stmt{"RawMetrics | take 10"}
	//kustoclient.Query(context.Background(), "testDB", stmt)
}

func TestClose(t *testing.T) {
	logger := zaptest.NewLogger(t)
	kustoclient := kusto.NewMockClient()
	ingestoptions := make([]ingest.FileOption, 2)
	ingestoptions[0] = ingest.FileFormat(ingest.MultiJSON)
	ingestoptions[1] = ingest.IngestionMappingRef(fmt.Sprintf("%s_mapping", strings.ToLower("RawMetrics")), ingest.MultiJSON)
	managedstreamingingest, _ := ingest.NewManaged(kustoclient, "testDB", "RawMetrics")

	adxMetricsProducer := &adxDataProducer{
		client:        kustoclient,
		managedingest: managedstreamingingest,
		ingestoptions: ingestoptions,
		logger:        logger,
	}
	err := adxMetricsProducer.Close(context.Background())
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
