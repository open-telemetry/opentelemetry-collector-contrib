package servicegraphprocessor

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
