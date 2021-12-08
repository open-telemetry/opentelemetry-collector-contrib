// Copyright  The OpenTelemetry Authors
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

package kafkaexporter

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	tagInstanceName, _ = tag.NewKey("name")
	tagInstanceType, _ = tag.NewKey("type")
	statMessageCount   = stats.Int64("kafka_exporter_messages", "Number of exported messages", stats.UnitDimensionless)
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
