package kafkaexporter

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	tagInstanceName, _ = tag.NewKey("name")
	tagInstanceType, _ = tag.NewKey("type")
	statMessageCount = stats.Int64("kafka_exporter_messages", "Number of exported messages", stats.UnitDimensionless)
)


func MetricViews() []*view.View {
	return []*view.View{
		{
			Name:        statMessageCount.Name(),
			Description: statMessageCount.Description(),
			Measure:     statMessageCount,
			TagKeys:     []tag.Key{tagInstanceName, tagInstanceType},
			Aggregation: view.Sum(),
		},
	}
}