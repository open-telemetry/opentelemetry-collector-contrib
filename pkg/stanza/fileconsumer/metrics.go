// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/config/configtelemetry"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	scopeName   = "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	receiverKey = "receiver"
	nameSep     = "_"
	typeStr     = "filelog"
)

var (
	receiverTagKey    = tag.MustNewKey(receiverKey)
	statFileReadDelay = stats.Int64("file_read_delay", "the time it takes to read data from a file", stats.UnitMilliseconds)
)

func init() {
	_ = view.Register(metricViews()...)
}

func BuildMetricName(typestr string, metric string) string {
	return receiverKey + nameSep + typestr + nameSep + metric
}

// MetricViews returns the metrics views related to fileconsumer.
func metricViews() []*view.View {
	receiverTagKeys := []tag.Key{receiverTagKey}

	distributionReadFileDelayView := &view.View{
		Name:        BuildMetricName(typeStr, statFileReadDelay.Name()),
		Measure:     statFileReadDelay,
		Description: statFileReadDelay.Description(),
		TagKeys:     receiverTagKeys,
		Aggregation: view.Distribution(10, 25, 50, 75, 100, 250, 500, 750, 1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000, 10000, 20000, 30000, 50000,
			100_000, 200_000, 300_000, 400_000, 500_000, 600_000, 700_000, 800_000, 900_000,
			1000_000, 2000_000, 3000_000, 4000_000, 5000_000, 6000_000, 7000_000, 8000_000, 9000_000),
	}

	return []*view.View{
		distributionReadFileDelayView,
	}
}

type fileConsumerTelemetry struct {
	level    configtelemetry.Level
	detailed bool
	useOtel  bool

	exportCtx context.Context

	receiverAttr  []attribute.KeyValue
	readFileDelay metric.Int64Histogram
}

func newFileConsumerTelemetry(set *rcvr.CreateSettings, useOtel bool) (*fileConsumerTelemetry, error) {
	exportCtx, err := tag.New(context.Background(), tag.Insert(receiverTagKey, set.ID.String()))
	if err != nil {
		return nil, err
	}

	bpt := &fileConsumerTelemetry{
		useOtel:      useOtel,
		receiverAttr: []attribute.KeyValue{attribute.String(receiverKey, set.ID.String())},
		exportCtx:    exportCtx,
		level:        set.MetricsLevel,
		detailed:     set.MetricsLevel == configtelemetry.LevelDetailed,
	}

	err = bpt.createOtelMetrics(set.MeterProvider)
	if err != nil {
		return nil, err
	}
	return bpt, nil
}

func (bpt *fileConsumerTelemetry) createOtelMetrics(mp metric.MeterProvider) error {
	if !bpt.useOtel {
		return nil
	}

	var err error
	meter := mp.Meter(scopeName)
	bpt.readFileDelay, err = meter.Int64Histogram(
		BuildMetricName(typeStr, statFileReadDelay.Name()),
		metric.WithDescription(statFileReadDelay.Description()),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}
	return nil
}

func (bpt *fileConsumerTelemetry) record(delay int64) {
	if bpt.useOtel {
		bpt.recordWithOtel(delay)
	} else {
		bpt.recordWithOC(delay)
	}
}

func (bpt *fileConsumerTelemetry) recordWithOC(delay int64) {
	if bpt.detailed {
		stats.Record(bpt.exportCtx, statFileReadDelay.M(delay))
	}
}

func (bpt *fileConsumerTelemetry) recordWithOtel(delay int64) {
	if bpt.detailed {
		bpt.readFileDelay.Record(bpt.exportCtx, delay, metric.WithAttributes(bpt.receiverAttr...))
	}
}
