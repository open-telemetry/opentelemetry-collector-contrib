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
	ms.Resize(len(rawMetrics))
	for i := 0; i < len(rawMetrics); i++ {
		rawMetricToPdata(rawMetrics[i], ms.At(i), startTime, now)
	}
	return pdm
}

func rawMetricToPdata(dm dotnet.Metric, pdm pdata.Metric, startTime, now time.Time) pdata.Metric {
	const metricNamePrefix = "dotnet."
	pdm.SetName(metricNamePrefix + dm.Name())
	pdm.SetDescription(dm.DisplayName())
	pdm.SetUnit(mapUnits(dm.DisplayUnits()))
	nowPD := pdata.TimestampFromTime(now)
	switch dm.CounterType() {
	case "Mean":
		pdm.SetDataType(pdata.MetricDataTypeDoubleGauge)
		dps := pdm.DoubleGauge().DataPoints()
		dp := dps.AppendEmpty()
		dp.SetTimestamp(nowPD)
		dp.SetValue(dm.Mean())
	case "Sum":
		pdm.SetDataType(pdata.MetricDataTypeDoubleSum)
		sum := pdm.DoubleSum()
		sum.SetAggregationTemporality(pdata.AggregationTemporalityDelta)
		dps := sum.DataPoints()
		dp := dps.AppendEmpty()
		dp.SetStartTimestamp(pdata.TimestampFromTime(startTime))
		dp.SetTimestamp(nowPD)
		dp.SetValue(dm.Increment())
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
