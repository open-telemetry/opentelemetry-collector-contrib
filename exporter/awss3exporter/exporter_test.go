// Copyright 2022, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awss3exporter

import (
	"context"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"testing"
	"time"
)

var testTimestamp = pcommon.Timestamp(time.Date(2022, 05, 17, 12, 30, 0, 0, time.UTC).UnixNano())

type TestWriter struct {
	t *testing.T
}

func (testWriter *TestWriter) WriteParquet(metrics []*ParquetMetric, ctx context.Context, config *Config) error {
	assert.Equal(testWriter.t, 1, len(metrics))
	_, foundMetric := metrics[0].Metrics["int_sum"]
	assert.Equal(testWriter.t, true, foundMetric)
	assert.Equal(testWriter.t, metrics[0].Metrics["int_sum"].Value.(float64), float64(10))
	return nil
}

func (testWriter *TestWriter) WriteJson(buf []byte, config *Config) error {
	return nil
}

func TestConsumeMetrics(t *testing.T) {
	config := createDefaultConfig()
	expConfig := config.(*Config)
	s3Exporter := &S3Exporter{
		config:           config,
		metricTranslator: newMetricTranslator(*expConfig),
		dataWriter:       &TestWriter{t: t},
		logger:           zap.NewNop(),
	}
	md := pmetric.NewMetrics()
	md.ResourceMetrics().EnsureCapacity(2)
	rm := md.ResourceMetrics().AppendEmpty()

	ilms := rm.ScopeMetrics()
	ilms.EnsureCapacity(2)
	ilm := ilms.AppendEmpty()

	metrics := ilm.Metrics()

	intSumMetric := metrics.AppendEmpty()
	intSumMetric.SetDataType(pmetric.MetricDataTypeSum)
	intSumMetric.SetName("int_sum")
	intSum := intSumMetric.Sum()
	intSumDataPoints := intSum.DataPoints()
	intSumDataPoint := intSumDataPoints.AppendEmpty()
	intSumDataPoint.SetIntVal(10)
	intSumDataPoint.SetTimestamp(testTimestamp)

	s3Exporter.ConsumeMetrics(context.Background(), md)
	assert.NotNil(t, md, "failed to create metrics")

}
