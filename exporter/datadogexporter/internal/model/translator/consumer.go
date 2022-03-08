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

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/translator"

import (
	"context"

	"github.com/DataDog/datadog-agent/pkg/quantile"
)

// MetricDataType is a timeseries-style metric type.
type MetricDataType int

const (
	// Gauge is the Datadog Gauge metric type.
	Gauge MetricDataType = iota
	// Count is the Datadog Count metric type.
	Count
)

// TimeSeriesConsumer is timeseries consumer.
type TimeSeriesConsumer interface {
	// ConsumeTimeSeries consumes a timeseries-style metric.
	ConsumeTimeSeries(
		ctx context.Context,
		dimensions *Dimensions,
		typ MetricDataType,
		timestamp uint64,
		value float64,
	)
}

// SketchConsumer is a pkg/quantile sketch consumer.
type SketchConsumer interface {
	// ConsumeSketch consumes a pkg/quantile-style sketch.
	ConsumeSketch(
		ctx context.Context,
		dimensions *Dimensions,
		timestamp uint64,
		sketch *quantile.Sketch,
	)
}

// Consumer is a metrics consumer.
type Consumer interface {
	TimeSeriesConsumer
	SketchConsumer
}

// HostConsumer is a hostname consumer.
// It is an optional interface that can be implemented by a Consumer.
type HostConsumer interface {
	// ConsumeHost consumes a hostname.
	ConsumeHost(host string)
}

// TagsConsumer is a tags consumer.
// It is an optional interface that can be implemented by a Consumer.
// Consumed tags are used for running metrics, and should represent
// some resource running a Collector (e.g. Fargate task).
type TagsConsumer interface {
	// ConsumeTag consumes a tag
	ConsumeTag(tag string)
}
