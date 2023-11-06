package deltatocumulativeprocessor

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

const (
	scopeName = "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"
	nameSep   = "/"

	processorKey = "processor"
	metricSep    = "_"

	reasonKey            = "reason"
	removedReasonExpired = "expired"
	removedReasonStale   = "stale"

	datapointDroppedReasonLate = "late"
)

var (
	statTimeseriesAdded   = stats.Int64("timeseries.added", "Number of timeseries that were added to tracking.", "span")
	statTimeseriesRemoved = stats.Int64("timeseries.removed", "Number of timeseries that were removed from tracking.", "span")
	statDatapointDropped  = stats.Int64("datapoint.dropped", "Number of datapoint that were dropped.", "datapoint")
	removedExpiredMutator = tag.Upsert(tag.MustNewKey(reasonKey), removedReasonExpired)
	removedStaleMutator   = tag.Upsert(tag.MustNewKey(reasonKey), removedReasonStale)
	droppedLateMutator    = tag.Upsert(tag.MustNewKey(reasonKey), datapointDroppedReasonLate)
)

func newTelemetry(ctx context.Context, settings *component.TelemetrySettings, level configtelemetry.Level, useOtelForMetrics bool) (*telemetry, error) {

	var err error

	meter := settings.MeterProvider.Meter(scopeName + nameSep + "metrics")
	timeseriesAdded, err := meter.Int64Counter(
		metadata.Type+metricSep+processorKey+metricSep+statTimeseriesAdded.Name(),
		metric.WithDescription(statTimeseriesAdded.Description()),
	)
	if err != nil {
		return nil, err
	}

	timeseriesRemoved, err := meter.Int64Counter(
		metadata.Type+metricSep+processorKey+metricSep+statTimeseriesRemoved.Name(),
		metric.WithDescription(statTimeseriesRemoved.Description()),
	)
	if err != nil {
		return nil, err
	}

	datapointDropped, err := meter.Int64Counter(
		metadata.Type+metricSep+processorKey+metricSep+statDatapointDropped.Name(),
		metric.WithDescription(statDatapointDropped.Description()),
	)
	if err != nil {
		return nil, err
	}

	return &telemetry{
		ctx:               ctx,
		settings:          settings,
		telemetryLevel:    level,
		attributes:        make([]attribute.KeyValue, 0),
		timeseriesAdded:   timeseriesAdded,
		timeseriesRemoved: timeseriesRemoved,
		datapointDropped:  datapointDropped,
		useOtelForMetrics: useOtelForMetrics,
	}, nil
}

type telemetry struct {
	ctx               context.Context
	attributes        []attribute.KeyValue
	settings          *component.TelemetrySettings
	telemetryLevel    configtelemetry.Level
	timeseriesAdded   metric.Int64Counter
	timeseriesRemoved metric.Int64Counter
	datapointDropped  metric.Int64Counter
	useOtelForMetrics bool
}

func (t *telemetry) recordNewTimeseries(signature string) {
	t.log("Tracking new metric", zap.String("signature", signature))
	if t.useOtelForMetrics {
		t.timeseriesAdded.Add(t.ctx, 1, metric.WithAttributes(t.attributes...))
	} else {
		stats.Record(t.ctx, statTimeseriesAdded.M(1))
	}
}

func (t *telemetry) recordDroppedDatapointLate(signature string) {
	t.log("Dropping late datapoint", zap.String("signature", signature))
	if t.useOtelForMetrics {
		t.datapointDropped.Add(t.ctx, 1, metric.WithAttributes(append(t.attributes, attribute.String(reasonKey, datapointDroppedReasonLate))...))
	} else {
		_ = stats.RecordWithTags(t.ctx, []tag.Mutator{droppedLateMutator}, statDatapointDropped.M(1))
	}
}

func (t *telemetry) recordExpiredTimeseries(signature string) {
	t.log("Removing expired timeseries", zap.String("signature", signature))
	if t.useOtelForMetrics {
		t.timeseriesRemoved.Add(t.ctx, 1, metric.WithAttributes(append(t.attributes, attribute.String(reasonKey, removedReasonExpired))...))
	} else {
		_ = stats.RecordWithTags(t.ctx, []tag.Mutator{removedExpiredMutator}, statTimeseriesRemoved.M(1))
	}
}

func (t *telemetry) recordStaleTimeseries(signature string) {
	t.log("Removing stale metric", zap.String("signature", signature))
	if t.useOtelForMetrics {
		t.timeseriesRemoved.Add(t.ctx, 1, metric.WithAttributes(append(t.attributes, attribute.String(reasonKey, removedReasonStale))...))
	} else {
		_ = stats.RecordWithTags(t.ctx, []tag.Mutator{removedStaleMutator}, statTimeseriesRemoved.M(1))
	}
}

func (t *telemetry) log(msg string, fields ...zap.Field) {
	switch t.telemetryLevel {
	case configtelemetry.LevelDetailed:
		t.settings.Logger.Info(msg, fields...)
	default:
		t.settings.Logger.Debug(msg, fields...)
	}
}

func deltaToCumulativeProcessorMetricViews(level configtelemetry.Level) []*view.View {
	if level == configtelemetry.LevelNone {
		return nil
	}

	timeseriesAddedView := &view.View{
		Name:        processorhelper.BuildCustomMetricName(metadata.Type, statTimeseriesAdded.Name()),
		Measure:     statTimeseriesAdded,
		Description: statTimeseriesAdded.Description(),
		Aggregation: view.Sum(),
	}

	timeseriesRemovedView := createViewWithReasonFromMeasure(statTimeseriesRemoved)

	datapointDroppedView := createViewWithReasonFromMeasure(statDatapointDropped)

	return []*view.View{
		timeseriesAddedView,
		timeseriesRemovedView,
		datapointDroppedView,
	}
}

func createViewWithReasonFromMeasure(measure *stats.Int64Measure) *view.View {
	return &view.View{
		Name:        processorhelper.BuildCustomMetricName(metadata.Type, measure.Name()),
		Measure:     measure,
		Description: measure.Description(),
		TagKeys: []tag.Key{
			tag.MustNewKey(reasonKey),
		},
		Aggregation: view.Sum(),
	}
}
