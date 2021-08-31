// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"time"

	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/dotnet"
)

func rawMetricsToPdata(rawMetrics []dotnet.Metric, startTime, now time.Time) pdata.Metrics {
	pdm := pdata.NewMetrics()
	rms := pdm.ResourceMetrics()
	rm := rms.AppendEmpty()
	rm.Resource().Attributes()
	ilms := rm.InstrumentationLibraryMetrics()
	ilm := ilms.AppendEmpty()
	ms := ilm.Metrics()
	ms.EnsureCapacity(len(rawMetrics))
	for i := 0; i < len(rawMetrics); i++ {
		rawMetricToPdata(rawMetrics[i], ms.AppendEmpty(), startTime, now)
	}
	return pdm
}

func rawMetricToPdata(dm dotnet.Metric, pdm pdata.Metric, startTime, now time.Time) pdata.Metric {
	const metricNamePrefix = "dotnet."
	pdm.SetName(metricNamePrefix + dm.Name())
	pdm.SetDescription(dm.DisplayName())
	pdm.SetUnit(mapUnits(dm.DisplayUnits()))
	nowPD := pdata.NewTimestampFromTime(now)
	switch dm.CounterType() {
	case "Mean":
		pdm.SetDataType(pdata.MetricDataTypeGauge)
		dps := pdm.Gauge().DataPoints()
		dp := dps.AppendEmpty()
		dp.SetTimestamp(nowPD)
		dp.SetDoubleVal(dm.Mean())
	case "Sum":
		pdm.SetDataType(pdata.MetricDataTypeSum)
		sum := pdm.Sum()
		sum.SetAggregationTemporality(pdata.AggregationTemporalityDelta)
		dps := sum.DataPoints()
		dp := dps.AppendEmpty()
		dp.SetStartTimestamp(pdata.NewTimestampFromTime(startTime))
		dp.SetTimestamp(nowPD)
		dp.SetDoubleVal(dm.Increment())
	}
	return pdm
}

// mapUnits overrides a dotnet-provided units string with one that conforms to
// otel if necessary. "MB" and "%" returned by System.Runtime are already
// conforming so are left unchanged.
func mapUnits(units string) string {
	// do we want to make this mapping configurable?
	switch units {
	case "B":
		return "By"
	}
	return units
}
