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

package servicegraphprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor"

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/obsreport"
)

var (
	statDroppedSpans = stats.Int64("dropped_spans", "Number of spans dropped when trying to add edges", stats.UnitDimensionless)
	statTotalEdges   = stats.Int64("total_edges", "Total number of unique edges", stats.UnitDimensionless)
	statExpiredEdges = stats.Int64("expired_edges", "Number of edges that expired before finding its matching span", stats.UnitDimensionless)
)

func serviceGraphProcessorViews() []*view.View {
	droppedSpansView := &view.View{
		Name:        obsreport.BuildProcessorCustomMetricName(typeStr, statDroppedSpans.Name()),
		Description: statDroppedSpans.Description(),
		Measure:     statDroppedSpans,
		Aggregation: view.Count(),
	}
	totalEdgesView := &view.View{
		Name:        obsreport.BuildProcessorCustomMetricName(typeStr, statTotalEdges.Name()),
		Description: statTotalEdges.Description(),
		Measure:     statTotalEdges,
		Aggregation: view.Count(),
	}
	expiredEdgesView := &view.View{
		Name:        obsreport.BuildProcessorCustomMetricName(typeStr, statExpiredEdges.Name()),
		Description: statExpiredEdges.Description(),
		Measure:     statExpiredEdges,
		Aggregation: view.Count(),
	}

	return []*view.View{
		droppedSpansView,
		totalEdgesView,
		expiredEdgesView,
	}
}
