// Copyright 2020, OpenTelemetry Authors
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

package alibabacloudlogserviceexporter

import (
	"context"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/testutil/metricstestutil"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
)

func TestNewMetricsExporter(t *testing.T) {
	got, err := newMetricsExporter(zap.NewNop(), &Config{
		Endpoint: "us-west-1.log.aliyuncs.com",
		Project:  "demo-project",
		Logstore: "demo-logstore",
	})
	assert.NoError(t, err)
	require.NotNil(t, got)

	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)

	doubleVal := 1234.5678
	doublePt := metricstestutil.Double(tsUnix, doubleVal)

	// This will put trace data to send buffer and return success.
	err = got.ConsumeMetrics(context.Background(), internaldata.OCToMetrics(consumerdata.MetricsData{
		Metrics: []*metricspb.Metric{
			metricstestutil.Gauge("gauge_double_with_dims", nil, metricstestutil.Timeseries(tsUnix, nil, doublePt)),
		},
	}))
	assert.NoError(t, err)
}

func TestNewFailsWithEmptyMetricsExporterName(t *testing.T) {
	got, err := newMetricsExporter(zap.NewNop(), &Config{})
	assert.Error(t, err)
	require.Nil(t, got)
}
