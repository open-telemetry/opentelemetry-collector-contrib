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

package loadbalancingexporter

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/obsreport"
)

var (
	mNumResolutions    = stats.Int64("num_resolutions", "Number of times the resolver triggered a new resolutions", stats.UnitDimensionless)
	mNumBackendUpdates = stats.Int64("num_backend_updates", "Number of times the list of backends was updated", stats.UnitDimensionless)
	mNumBackends       = stats.Int64("num_backends", "Current number of backends in use", stats.UnitDimensionless)
)

// MetricViews return the metrics views according to given telemetry level.
func MetricViews() []*view.View {
	legacyViews := []*view.View{
		{
			Name:        mNumResolutions.Name(),
			Measure:     mNumResolutions,
			Description: mNumResolutions.Description(),
			Aggregation: view.Count(),
		},
		{
			Name:        mNumBackendUpdates.Name(),
			Measure:     mNumBackendUpdates,
			Description: mNumBackendUpdates.Description(),
			Aggregation: view.Count(),
		},
		{
			Name:        mNumBackends.Name(),
			Measure:     mNumBackends,
			Description: mNumBackends.Description(),
			Aggregation: view.LastValue(),
		},
	}

	return obsreport.ProcessorMetricViews(string(typeStr), legacyViews)
}
