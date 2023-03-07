// Copyright The OpenTelemetry Authors
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

package otel2influx_test

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/otel2influx"
)

type mockPoint struct {
	measurement string
	tags        map[string]string
	fields      map[string]interface{}
	ts          time.Time
	vType       common.InfluxMetricValueType
}

var _ otel2influx.InfluxWriter = &MockInfluxWriter{}
var _ otel2influx.InfluxWriterBatch = &MockInfluxWriterBatch{}

type MockInfluxWriter struct {
	points []mockPoint
}

func (w *MockInfluxWriter) NewBatch() otel2influx.InfluxWriterBatch {
	return &MockInfluxWriterBatch{w: w}
}

func (w *MockInfluxWriter) WritePoint(_ context.Context, measurement string, tags map[string]string, fields map[string]interface{}, ts time.Time, vType common.InfluxMetricValueType) error {
	return nil
}

type MockInfluxWriterBatch struct {
	w *MockInfluxWriter
}

func (b *MockInfluxWriterBatch) WritePoint(ctx context.Context, measurement string, tags map[string]string, fields map[string]interface{}, ts time.Time, vType common.InfluxMetricValueType) error {
	b.w.points = append(b.w.points, mockPoint{
		measurement: measurement,
		tags:        tags,
		fields:      fields,
		ts:          ts,
		vType:       vType,
	})
	return nil
}

func (b *MockInfluxWriterBatch) FlushBatch(ctx context.Context) error {
	return nil
}

var (
	timestamp      = pcommon.Timestamp(1395066363000000123)
	startTimestamp = pcommon.Timestamp(1395066363000000001)
)
