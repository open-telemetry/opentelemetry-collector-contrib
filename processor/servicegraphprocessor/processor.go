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
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor/internal/store"
	"go.opencensus.io/stats"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
)

const (
	metricKeySeparator = string(byte(0))
)

var (
	maxDuration   = time.Duration(math.MaxInt64)
	maxDurationMs = durationToMillis(maxDuration)

	defaultLatencyHistogramBucketsMs = []float64{
		2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000, maxDurationMs,
	}
)

type metricSeries struct {
	dimensions pcommon.Map
	// TODO: Remove stale series from the processor
	lastUpdated int64 // Used to remove stale series
}

var _ component.TracesProcessor = (*processor)(nil)

type processor struct {
	config          *Config
	logger          *zap.Logger
	nextConsumer    consumer.Traces
	metricsExporter consumer.Metrics

	store store.Store

	startTime time.Time

	seriesMutex                    sync.Mutex
	reqTotal                       map[string]int64
	reqFailedTotal                 map[string]int64
	reqDurationSecondsSum          map[string]float64
	reqDurationSecondsCount        map[string]uint64
	reqDurationBounds              []float64
	reqDurationSecondsBucketCounts map[string][]uint64

	keyToMetric map[string]metricSeries
}

func newProcessor(logger *zap.Logger, config config.Processor, nextConsumer consumer.Traces) (*processor, error) {
	logger.Info("Building servicegraphsprocessor")

	pConfig := config.(*Config)

	bounds := defaultLatencyHistogramBucketsMs
	if pConfig.LatencyHistogramBuckets != nil {
		bounds = mapDurationsToMillis(pConfig.LatencyHistogramBuckets)

		// "Catch-all" bucket.
		// TODO: Revisit catch-all bucket, it's very problematic for Prometheus
		//  See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/2838
		if bounds[len(bounds)-1] != maxDurationMs {
			bounds = append(bounds, maxDurationMs)
		}
	}

	p := &processor{
		config:                         pConfig,
		logger:                         logger,
		nextConsumer:                   nextConsumer,
		startTime:                      time.Now(),
		reqTotal:                       make(map[string]int64),
		reqFailedTotal:                 make(map[string]int64),
		reqDurationSecondsSum:          make(map[string]float64),
		reqDurationSecondsCount:        make(map[string]uint64),
		reqDurationBounds:              bounds,
		reqDurationSecondsBucketCounts: make(map[string][]uint64),
		keyToMetric:                    make(map[string]metricSeries),
	}

	return p, nil
}

func (p *processor) Start(_ context.Context, host component.Host) error {
	p.logger.Info("Starting servicegraphsprocessor")

	store, err := store.NewStore(p.config.Store, p.onComplete, p.onExpire)
	if err != nil {
		return fmt.Errorf("failed to instantiate store: %w", err)
	}
	p.store = store

	exporters := host.GetExporters()

	// The available list of exporters come from any configured metrics pipelines' exporters.
	for k, exp := range exporters[config.MetricsDataType] {
		metricsExp, ok := exp.(component.MetricsExporter)
		if k.String() == p.config.MetricsExporter && ok {
			p.metricsExporter = metricsExp
			break
		}
	}

	if p.metricsExporter == nil {
		return fmt.Errorf("failed to find metrics exporter: %s",
			p.config.MetricsExporter)
	}

	p.logger.Info("Started servicegraphsprocessor")
	return nil
}

func (p *processor) Shutdown(_ context.Context) error {
	p.logger.Info("Shutting down servicegraphsprocessor")
	return nil
}

func (p *processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (p *processor) ConsumeTraces(ctx context.Context, td ptrace.Traces) (err error) {
	if err := p.aggregateMetrics(ctx, td); err != nil {
		return fmt.Errorf("failed to aggregate md: %w", err)
	}

	md := p.buildMetrics()
	// Firstly, export md to avoid being impacted by downstream trace processor errors/latency.
	if err := p.metricsExporter.ConsumeMetrics(ctx, md); err != nil {
		return err
	}

	return p.nextConsumer.ConsumeTraces(ctx, td)
}

func (p *processor) aggregateMetrics(ctx context.Context, td ptrace.Traces) (err error) {
	var (
		isNew             bool
		totalDroppedSpans int
	)

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rSpans := rss.At(i)

		rAttributes := rSpans.Resource().Attributes()

		serviceName, ok := findServiceName(rAttributes)
		if !ok {
			// If service.name doesn't exist, skip processing this trace
			continue
		}

		scopeSpans := rSpans.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				switch span.Kind().String() {
				case v1.Span_SPAN_KIND_CLIENT.String():
					traceID := span.TraceID().HexString()
					key := buildEdgeKey(traceID, span.SpanID().HexString())
					isNew, err = p.store.UpsertEdge(key, func(e *store.Edge) {
						e.TraceID = traceID
						e.ClientService = serviceName
						e.ClientLatency = float64(span.EndTimestamp()-span.StartTimestamp()) / float64(time.Millisecond.Nanoseconds())
						e.Failed = e.Failed || span.Status().Code() == 2
						p.upsertDimensions(e.Dimensions, rAttributes, span.Attributes())
					})
				case v1.Span_SPAN_KIND_SERVER.String():
					traceID := span.TraceID().HexString()
					key := buildEdgeKey(traceID, span.ParentSpanID().HexString())
					isNew, err = p.store.UpsertEdge(key, func(e *store.Edge) {
						e.TraceID = traceID
						e.ServerService = serviceName
						e.ServerLatency = float64(span.EndTimestamp()-span.StartTimestamp()) / float64(time.Millisecond.Nanoseconds())
						e.Failed = e.Failed || span.Status().Code() == 2
						p.upsertDimensions(e.Dimensions, rAttributes, span.Attributes())
					})
				default:
					// this span is not part of an edge
					continue
				}

				if errors.Is(err, store.ErrTooManyItems) {
					totalDroppedSpans++
					stats.Record(ctx, statDroppedSpans.M(1))
					continue
				}

				// UpsertEdge will only return ErrTooManyItems
				if err != nil {
					return err
				}

				if isNew {
					stats.Record(ctx, statTotalEdges.M(1))
				}
			}
		}
	}
	return nil
}

func (p *processor) upsertDimensions(m map[string]string, resourceAttr pcommon.Map, spanAttr pcommon.Map) {
	for _, dim := range p.config.Dimensions {
		if v, ok := findAttributeValue(dim, resourceAttr, spanAttr); ok {
			m[dim] = v
		}
	}
}

func (p *processor) onComplete(e *store.Edge) {
	p.logger.Debug("edge completed")
	p.aggregateMetricsForEdge(e)
}

func (p *processor) onExpire(*store.Edge) {
	p.logger.Debug("edge expired")
	stats.Record(context.Background(), statExpiredEdges.M(1))
}

func (p *processor) aggregateMetricsForEdge(e *store.Edge) {
	metricKey := p.buildMetricKey(e.ClientService, e.ServerService, e.Dimensions)
	dimensions := buildDimensions(e)

	// TODO: Consider configuring server or client latency
	duration := e.ServerLatency

	p.seriesMutex.Lock()
	defer p.seriesMutex.Unlock()
	p.updateSeries(metricKey, dimensions)
	p.updateCountMetrics(metricKey)
	p.updateDurationMetrics(metricKey, duration)
}

func (p *processor) updateSeries(key string, dimensions pcommon.Map) {
	if series, ok := p.keyToMetric[key]; ok {
		series.lastUpdated = time.Now().UnixMilli()
		return
	}

	p.keyToMetric[key] = metricSeries{
		dimensions:  dimensions,
		lastUpdated: time.Now().UnixMilli(),
	}
}

func (p *processor) dimensionsForSeries(key string) (pcommon.Map, bool) {
	if series, ok := p.keyToMetric[key]; ok {
		return series.dimensions, true
	}

	return pcommon.Map{}, false
}

func (p *processor) updateCountMetrics(key string) {
	p.reqTotal[key]++
}

func (p *processor) updateDurationMetrics(key string, duration float64) {
	index := sort.SearchFloat64s(p.reqDurationBounds, duration) // Search bucket index
	if _, ok := p.reqDurationSecondsBucketCounts[key]; !ok {
		p.reqDurationSecondsBucketCounts[key] = make([]uint64, len(p.reqDurationBounds))
	}
	p.reqDurationSecondsSum[key] += duration
	p.reqDurationSecondsCount[key]++
	p.reqDurationSecondsBucketCounts[key][index]++
}

func buildDimensions(e *store.Edge) pcommon.Map {
	dims := pcommon.NewMap()
	dims.UpsertString("client", e.ClientService)
	dims.UpsertString("server", e.ServerService)
	dims.UpsertBool("failed", e.Failed)
	for k, v := range e.Dimensions {
		dims.UpsertString(k, v)
	}
	return dims
}

func (p *processor) buildMetrics() pmetric.Metrics {
	m := pmetric.NewMetrics()
	ilm := m.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("servicegraphprocessor")

	// Obtain write lock to reset data
	p.seriesMutex.Lock()
	defer p.seriesMutex.Unlock()

	if err := p.collectCountMetrics(ilm); err != nil {
		return m
	}

	if err := p.collectLatencyMetrics(ilm); err != nil {
		return m
	}

	return m
}

func (p *processor) collectCountMetrics(ilm pmetric.ScopeMetrics) error {
	for key, c := range p.reqTotal {
		mCount := ilm.Metrics().AppendEmpty()
		mCount.SetDataType(pmetric.MetricDataTypeSum)
		mCount.SetName("request_total")
		mCount.Sum().SetIsMonotonic(true)
		// TODO: Support other aggregation temporalities
		mCount.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)

		dpCalls := mCount.Sum().DataPoints().AppendEmpty()
		dpCalls.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpCalls.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpCalls.SetIntVal(c)

		dimensions, ok := p.dimensionsForSeries(key)
		if !ok {
			return fmt.Errorf("failed to find dimensions for key %s", key)
		}

		dimensions.CopyTo(dpCalls.Attributes())
	}

	return nil
}

func (p *processor) collectLatencyMetrics(ilm pmetric.ScopeMetrics) error {
	for key := range p.reqDurationSecondsCount {
		mDuration := ilm.Metrics().AppendEmpty()
		mDuration.SetDataType(pmetric.MetricDataTypeHistogram)
		mDuration.SetName("request_duration_seconds")
		// TODO: Support other aggregation temporalities
		mDuration.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)

		timestamp := pcommon.NewTimestampFromTime(time.Now())

		dpDuration := mDuration.Histogram().DataPoints().AppendEmpty()
		dpDuration.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpDuration.SetTimestamp(timestamp)
		dpDuration.SetMExplicitBounds(p.reqDurationBounds)
		dpDuration.SetMBucketCounts(p.reqDurationSecondsBucketCounts[key])
		dpDuration.SetCount(p.reqDurationSecondsCount[key])
		dpDuration.SetSum(p.reqDurationSecondsSum[key])

		// TODO: Support exemplars

		dimensions, ok := p.dimensionsForSeries(key)
		if !ok {
			return fmt.Errorf("failed to find dimensions for key %s", key)
		}

		dimensions.CopyTo(dpDuration.Attributes())
	}
	return nil
}

func (p *processor) buildMetricKey(clientName, serverName string, edgeDimensions map[string]string) string {
	var metricKey strings.Builder
	metricKey.WriteString(clientName + metricKeySeparator + serverName)

	for _, dimName := range p.config.Dimensions {
		dim, ok := edgeDimensions[dimName]
		if !ok {
			continue
		}
		metricKey.WriteString(metricKeySeparator + dim)
	}

	return metricKey.String()
}

func buildEdgeKey(k1, k2 string) string {
	var b strings.Builder
	b.WriteString(k1)
	b.WriteString("-")
	b.WriteString(k2)
	return b.String()
}

// durationToMillis converts the given duration to the number of milliseconds it represents.
// Note that this can return sub-millisecond (i.e. < 1ms) values as well.
func durationToMillis(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / float64(time.Millisecond.Nanoseconds())
}

func mapDurationsToMillis(vs []time.Duration) []float64 {
	vsm := make([]float64, len(vs))
	for i, v := range vs {
		vsm[i] = durationToMillis(v)
	}
	return vsm
}
