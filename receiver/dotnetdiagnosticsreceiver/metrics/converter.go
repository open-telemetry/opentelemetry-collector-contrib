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
	"go.opentelemetry.io/collector/consumer/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/dotnet"
)

func rawMetricsToPdata(rawMetrics []dotnet.Metric) pdata.Metrics {
	metrics := pdata.NewMetrics()
	rms := metrics.ResourceMetrics()
	rms.Resize(1)
	rm := rms.At(0)
	ilms := rm.InstrumentationLibraryMetrics()
	ilms.Resize(1)
	ilm := ilms.At(0)
	ms := ilm.Metrics()
	ms.Resize(len(rawMetrics))
	for i := 0; i < len(rawMetrics); i++ {
		rawMetricToPdata(rawMetrics[i], ms.At(i))
	}
	return metrics
}

func rawMetricToPdata(rm dotnet.Metric, pdm pdata.Metric) pdata.Metric {
	pdm.SetName(rm.Name())
	switch rm.CounterType() {
	case "Mean":
		pdm.SetDataType(pdata.MetricDataTypeDoubleGauge)
		dps := pdm.DoubleGauge().DataPoints()
		dps.Resize(1)
		dps.At(0).SetValue(rm.Mean())
	case "Sum":
		pdm.SetDataType(pdata.MetricDataTypeDoubleSum)
		dps := pdm.DoubleSum().DataPoints()
		dps.Resize(1)
		dps.At(0).SetValue(rm.Increment())
	}
	return pdm
}
