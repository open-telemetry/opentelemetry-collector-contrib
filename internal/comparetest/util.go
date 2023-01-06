// Copyright The OpenTelemetry Authors
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

package comparetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/comparetest"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/pdatautil"
)

func metricsByName(metricSlice pmetric.MetricSlice) map[string]pmetric.Metric {
	byName := make(map[string]pmetric.Metric, metricSlice.Len())
	for i := 0; i < metricSlice.Len(); i++ {
		a := metricSlice.At(i)
		byName[a.Name()] = a
	}
	return byName
}

func getDataPointSlice(metric pmetric.Metric) pmetric.NumberDataPointSlice {
	var dataPointSlice pmetric.NumberDataPointSlice
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dataPointSlice = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dataPointSlice = metric.Sum().DataPoints()
	default:
		panic(fmt.Sprintf("data type not supported: %s", metric.Type()))
	}
	return dataPointSlice
}

func sortInstrumentationLibrary(a, b pmetric.ScopeMetrics) bool {
	if a.SchemaUrl() < b.SchemaUrl() {
		return true
	}
	if a.Scope().Name() < b.Scope().Name() {
		return true
	}
	if a.Scope().Version() < b.Scope().Version() {
		return true
	}
	return false
}

func sortResourceMetrics(a, b pmetric.ResourceMetrics) bool {
	if a.SchemaUrl() < b.SchemaUrl() {
		return true
	}
	return pdatautil.MapHash(a.Resource().Attributes()) < pdatautil.MapHash(b.Resource().Attributes())
}

func sortMetricSlice(a, b pmetric.Metric) bool {
	return a.Name() < b.Name()
}

func sortResourceLogs(a, b plog.ResourceLogs) bool {
	if a.SchemaUrl() < b.SchemaUrl() {
		return true
	}
	return pdatautil.MapHash(a.Resource().Attributes()) < pdatautil.MapHash(b.Resource().Attributes())
}

func sortLogsInstrumentationLibrary(a, b plog.ScopeLogs) bool {
	if a.SchemaUrl() < b.SchemaUrl() {
		return true
	}
	if a.Scope().Name() < b.Scope().Name() {
		return true
	}
	if a.Scope().Version() < b.Scope().Version() {
		return true
	}
	return false
}

func sortLogRecordSlice(a, b plog.LogRecord) bool {
	return pdatautil.MapHash(a.Attributes()) < pdatautil.MapHash(b.Attributes())
}
