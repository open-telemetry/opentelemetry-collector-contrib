// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor"

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	semconv "go.opentelemetry.io/collector/semconv/v1.13.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor/internal/store"
)

const (
	metricKeySeparator = string(byte(0))
	clientKind         = "client"
	serverKind         = "server"
)

var (
	defaultLatencyHistogramBucketsMs = []float64{
		2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000,
	}
	defaultPeerAttributes = []string{
		semconv.AttributeDBName, semconv.AttributeNetSockPeerAddr, semconv.AttributeNetPeerName, semconv.AttributeRPCService, semconv.AttributeNetSockPeerName, semconv.AttributeNetPeerName, semconv.AttributeHTTPURL, semconv.AttributeHTTPTarget,
	}
)

type metricSeries struct {
	dimensions  pcommon.Map
	lastUpdated int64 // Used to remove stale series
}

var _ processor.Traces = (*serviceGraphProcessor)(nil)

type serviceGraphProcessor struct {
	config          *Config
	logger          *zap.Logger
	metricsConsumer consumer.Metrics
	tracesConsumer  consumer.Traces

	store *store.Store

	startTime time.Time

	seriesMutex                    sync.Mutex
	reqTotal                       map[string]int64
	reqFailedTotal                 map[string]int64
	reqDurationSecondsSum          map[string]float64
	reqDurationSecondsCount        map[string]uint64
	reqDurationBounds              []float64
	reqDurationSecondsBucketCounts map[string][]uint64

	metricMutex sync.RWMutex
	keyToMetric map[string]metricSeries

	shutdownCh chan interface{}
}

func newProcessor(logger *zap.Logger, config component.Config) *serviceGraphProcessor {
	pConfig := config.(*Config)

	bounds := defaultLatencyHistogramBucketsMs
	if pConfig.LatencyHistogramBuckets != nil {
		bounds = mapDurationsToMillis(pConfig.LatencyHistogramBuckets)
	}

	if pConfig.CacheLoop <= 0 {
		pConfig.CacheLoop = time.Minute
	}

	if pConfig.StoreExpirationLoop <= 0 {
		pConfig.StoreExpirationLoop = 2 * time.Second
	}

	if pConfig.VirtualNodePeerAttributes == nil {
		pConfig.VirtualNodePeerAttributes = defaultPeerAttributes
	}

	return &serviceGraphProcessor{
		config:                         pConfig,
		logger:                         logger,
		startTime:                      time.Now(),
		reqTotal:                       make(map[string]int64),
		reqFailedTotal:                 make(map[string]int64),
		reqDurationSecondsSum:          make(map[string]float64),
		reqDurationSecondsCount:        make(map[string]uint64),
		reqDurationBounds:              bounds,
		reqDurationSecondsBucketCounts: make(map[string][]uint64),
		keyToMetric:                    make(map[string]metricSeries),
		shutdownCh:                     make(chan interface{}),
	}
}

func (p *serviceGraphProcessor) Start(_ context.Context, host component.Host) error {
	p.store = store.NewStore(p.config.Store.TTL, p.config.Store.MaxItems, p.onComplete, p.onExpire)

	if p.metricsConsumer == nil {
		exporters := host.GetExporters() //nolint:staticcheck

		// The available list of exporters come from any configured metrics pipelines' exporters.
		for k, exp := range exporters[component.DataTypeMetrics] {
			metricsExp, ok := exp.(exporter.Metrics)
			if k.String() == p.config.MetricsExporter && ok {
				p.metricsConsumer = metricsExp
				break
			}
		}

		if p.metricsConsumer == nil {
			return fmt.Errorf("failed to find metrics exporter: %s",
				p.config.MetricsExporter)
		}
	}

	go p.cacheLoop(p.config.CacheLoop)

	go p.storeExpirationLoop(p.config.StoreExpirationLoop)

	if p.tracesConsumer == nil {
		p.logger.Info("Started servicegraphconnector")
	} else {
		p.logger.Info("Started servicegraphprocessor")
	}
	return nil
}

func (p *serviceGraphProcessor) Shutdown(_ context.Context) error {
	if p.tracesConsumer == nil {
		p.logger.Info("Shutting down servicegraphconnector")
	} else {
		p.logger.Info("Shutting down servicegraphprocessor")
	}
	close(p.shutdownCh)
	return nil
}

func (p *serviceGraphProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (p *serviceGraphProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if err := p.aggregateMetrics(ctx, td); err != nil {
		return fmt.Errorf("failed to aggregate metrics: %w", err)
	}

	md, err := p.buildMetrics()
	if err != nil {
		return fmt.Errorf("failed to build metrics: %w", err)
	}

	// Skip empty metrics.
	if md.MetricCount() == 0 {
		return nil
	}

	// true when p is a connector
	if p.tracesConsumer == nil {
		return p.metricsConsumer.ConsumeMetrics(ctx, md)
	}

	// Firstly, export md to avoid being impacted by downstream trace serviceGraphProcessor errors/latency.
	if err := p.metricsConsumer.ConsumeMetrics(ctx, md); err != nil {
		return err
	}

	return p.tracesConsumer.ConsumeTraces(ctx, td)
}

func (p *serviceGraphProcessor) aggregateMetrics(ctx context.Context, td ptrace.Traces) (err error) {
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

				connectionType := store.Unknown

				switch span.Kind() {
				case ptrace.SpanKindProducer:
					// override connection type and continue processing as span kind client
					connectionType = store.MessagingSystem
					fallthrough
				case ptrace.SpanKindClient:
					traceID := span.TraceID()
					key := store.NewKey(traceID, span.SpanID())
					isNew, err = p.store.UpsertEdge(key, func(e *store.Edge) {
						e.TraceID = traceID
						e.ConnectionType = connectionType
						e.ClientService = serviceName
						e.ClientLatencySec = float64(span.EndTimestamp()-span.StartTimestamp()) / float64(time.Millisecond.Nanoseconds())
						e.Failed = e.Failed || span.Status().Code() == ptrace.StatusCodeError
						p.upsertDimensions(clientKind, e.Dimensions, rAttributes, span.Attributes())

						if virtualNodeFeatureGate.IsEnabled() {
							p.upsertPeerAttributes(p.config.VirtualNodePeerAttributes, e.Peer, span.Attributes())
						}

						// A database request will only have one span, we don't wait for the server
						// span but just copy details from the client span
						if dbName, ok := findAttributeValue(semconv.AttributeDBName, rAttributes, span.Attributes()); ok {
							e.ConnectionType = store.Database
							e.ServerService = dbName
							e.ServerLatencySec = float64(span.EndTimestamp()-span.StartTimestamp()) / float64(time.Millisecond.Nanoseconds())
						}
					})
				case ptrace.SpanKindConsumer:
					// override connection type and continue processing as span kind server
					connectionType = store.MessagingSystem
					fallthrough
				case ptrace.SpanKindServer:
					traceID := span.TraceID()
					key := store.NewKey(traceID, span.ParentSpanID())
					isNew, err = p.store.UpsertEdge(key, func(e *store.Edge) {
						e.TraceID = traceID
						e.ConnectionType = connectionType
						e.ServerService = serviceName
						e.ServerLatencySec = float64(span.EndTimestamp()-span.StartTimestamp()) / float64(time.Millisecond.Nanoseconds())
						e.Failed = e.Failed || span.Status().Code() == ptrace.StatusCodeError
						p.upsertDimensions(serverKind, e.Dimensions, rAttributes, span.Attributes())
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

func (p *serviceGraphProcessor) upsertDimensions(kind string, m map[string]string, resourceAttr pcommon.Map, spanAttr pcommon.Map) {
	for _, dim := range p.config.Dimensions {
		if v, ok := findAttributeValue(dim, resourceAttr, spanAttr); ok {
			m[kind+"_"+dim] = v
		}
	}
}

func (p *serviceGraphProcessor) upsertPeerAttributes(m []string, peers map[string]string, spanAttr pcommon.Map) {
	for _, s := range m {
		if v, ok := findAttributeValue(s, spanAttr); ok {
			peers[s] = v
			break
		}
	}
}

func (p *serviceGraphProcessor) onComplete(e *store.Edge) {
	p.logger.Debug(
		"edge completed",
		zap.String("client_service", e.ClientService),
		zap.String("server_service", e.ServerService),
		zap.String("connection_type", string(e.ConnectionType)),
		zap.Stringer("trace_id", e.TraceID),
	)
	p.aggregateMetricsForEdge(e)
}

func (p *serviceGraphProcessor) onExpire(e *store.Edge) {
	p.logger.Debug(
		"edge expired",
		zap.String("client_service", e.ClientService),
		zap.String("server_service", e.ServerService),
		zap.String("connection_type", string(e.ConnectionType)),
		zap.Stringer("trace_id", e.TraceID),
	)

	stats.Record(context.Background(), statExpiredEdges.M(1))

	if virtualNodeFeatureGate.IsEnabled() {
		e.ConnectionType = store.VirtualNode
		if len(e.ClientService) == 0 && e.Key.SpanIDIsEmpty() {
			e.ClientService = "user"
			p.onComplete(e)
		}

		if len(e.ServerService) == 0 {
			e.ServerService = p.getPeerHost(p.config.VirtualNodePeerAttributes, e.Peer)
			p.onComplete(e)
		}
	}
}

func (p *serviceGraphProcessor) aggregateMetricsForEdge(e *store.Edge) {
	metricKey := p.buildMetricKey(e.ClientService, e.ServerService, string(e.ConnectionType), e.Dimensions)
	dimensions := buildDimensions(e)

	// TODO: Consider configuring server or client latency
	duration := e.ServerLatencySec

	p.seriesMutex.Lock()
	defer p.seriesMutex.Unlock()
	p.updateSeries(metricKey, dimensions)
	p.updateCountMetrics(metricKey)
	if e.Failed {
		p.updateErrorMetrics(metricKey)
	}
	p.updateDurationMetrics(metricKey, duration)
}

func (p *serviceGraphProcessor) updateSeries(key string, dimensions pcommon.Map) {
	p.metricMutex.Lock()
	defer p.metricMutex.Unlock()
	// Overwrite the series if it already exists
	p.keyToMetric[key] = metricSeries{
		dimensions:  dimensions,
		lastUpdated: time.Now().UnixMilli(),
	}
}

func (p *serviceGraphProcessor) dimensionsForSeries(key string) (pcommon.Map, bool) {
	p.metricMutex.RLock()
	defer p.metricMutex.RUnlock()
	if series, ok := p.keyToMetric[key]; ok {
		return series.dimensions, true
	}

	return pcommon.Map{}, false
}

func (p *serviceGraphProcessor) updateCountMetrics(key string) { p.reqTotal[key]++ }

func (p *serviceGraphProcessor) updateErrorMetrics(key string) { p.reqFailedTotal[key]++ }

func (p *serviceGraphProcessor) updateDurationMetrics(key string, duration float64) {
	index := sort.SearchFloat64s(p.reqDurationBounds, duration) // Search bucket index
	if _, ok := p.reqDurationSecondsBucketCounts[key]; !ok {
		p.reqDurationSecondsBucketCounts[key] = make([]uint64, len(p.reqDurationBounds)+1)
	}
	p.reqDurationSecondsSum[key] += duration
	p.reqDurationSecondsCount[key]++
	p.reqDurationSecondsBucketCounts[key][index]++
}

func buildDimensions(e *store.Edge) pcommon.Map {
	dims := pcommon.NewMap()
	dims.PutStr("client", e.ClientService)
	dims.PutStr("server", e.ServerService)
	dims.PutStr("connection_type", string(e.ConnectionType))
	dims.PutBool("failed", e.Failed)
	for k, v := range e.Dimensions {
		dims.PutStr(k, v)
	}
	return dims
}

func (p *serviceGraphProcessor) buildMetrics() (pmetric.Metrics, error) {
	m := pmetric.NewMetrics()
	ilm := m.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("traces_service_graph")

	// Obtain write lock to reset data
	p.seriesMutex.Lock()
	defer p.seriesMutex.Unlock()

	if err := p.collectCountMetrics(ilm); err != nil {
		return m, err
	}

	if err := p.collectLatencyMetrics(ilm); err != nil {
		return m, err
	}

	return m, nil
}

func (p *serviceGraphProcessor) collectCountMetrics(ilm pmetric.ScopeMetrics) error {
	for key, c := range p.reqTotal {
		mCount := ilm.Metrics().AppendEmpty()
		mCount.SetName("traces_service_graph_request_total")
		mCount.SetEmptySum().SetIsMonotonic(true)
		// TODO: Support other aggregation temporalities
		mCount.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		dpCalls := mCount.Sum().DataPoints().AppendEmpty()
		dpCalls.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpCalls.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpCalls.SetIntValue(c)

		dimensions, ok := p.dimensionsForSeries(key)
		if !ok {
			return fmt.Errorf("failed to find dimensions for key %s", key)
		}

		dimensions.CopyTo(dpCalls.Attributes())
	}

	for key, c := range p.reqFailedTotal {
		mCount := ilm.Metrics().AppendEmpty()
		mCount.SetName("traces_service_graph_request_failed_total")
		mCount.SetEmptySum().SetIsMonotonic(true)
		// TODO: Support other aggregation temporalities
		mCount.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		dpCalls := mCount.Sum().DataPoints().AppendEmpty()
		dpCalls.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpCalls.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dpCalls.SetIntValue(c)

		dimensions, ok := p.dimensionsForSeries(key)
		if !ok {
			return fmt.Errorf("failed to find dimensions for key %s", key)
		}

		dimensions.CopyTo(dpCalls.Attributes())
	}

	return nil
}

func (p *serviceGraphProcessor) collectLatencyMetrics(ilm pmetric.ScopeMetrics) error {
	for key := range p.reqDurationSecondsCount {
		mDuration := ilm.Metrics().AppendEmpty()
		mDuration.SetName("traces_service_graph_request_duration_seconds")
		// TODO: Support other aggregation temporalities
		mDuration.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		timestamp := pcommon.NewTimestampFromTime(time.Now())

		dpDuration := mDuration.Histogram().DataPoints().AppendEmpty()
		dpDuration.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
		dpDuration.SetTimestamp(timestamp)
		dpDuration.ExplicitBounds().FromRaw(p.reqDurationBounds)
		dpDuration.BucketCounts().FromRaw(p.reqDurationSecondsBucketCounts[key])
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

func (p *serviceGraphProcessor) buildMetricKey(clientName, serverName, connectionType string, edgeDimensions map[string]string) string {
	var metricKey strings.Builder
	metricKey.WriteString(clientName + metricKeySeparator + serverName + metricKeySeparator + connectionType)

	for _, dimName := range p.config.Dimensions {
		dim, ok := edgeDimensions[dimName]
		if !ok {
			continue
		}
		metricKey.WriteString(metricKeySeparator + dim)
	}

	return metricKey.String()
}

// storeExpirationLoop periodically expires old entries from the store.
func (p *serviceGraphProcessor) storeExpirationLoop(d time.Duration) {
	t := time.NewTicker(d)
	for {
		select {
		case <-t.C:
			p.store.Expire()
		case <-p.shutdownCh:
			return
		}
	}
}

func (p *serviceGraphProcessor) getPeerHost(m []string, peers map[string]string) string {
	peerStr := "unknown"
	for _, s := range m {
		if peer, ok := peers[s]; ok {
			peerStr = peer
			break
		}
	}
	return peerStr
}

// cacheLoop periodically cleans the cache
func (p *serviceGraphProcessor) cacheLoop(d time.Duration) {
	t := time.NewTicker(d)
	for {
		select {
		case <-t.C:
			p.cleanCache()
		case <-p.shutdownCh:
			return
		}
	}

}

// cleanCache removes series that have not been updated in 15 minutes
func (p *serviceGraphProcessor) cleanCache() {
	var staleSeries []string
	p.metricMutex.RLock()
	for key, series := range p.keyToMetric {
		if series.lastUpdated+15*time.Minute.Milliseconds() < time.Now().UnixMilli() {
			staleSeries = append(staleSeries, key)
		}
	}
	p.metricMutex.RUnlock()

	p.metricMutex.Lock()
	for _, key := range staleSeries {
		delete(p.keyToMetric, key)
	}
	p.metricMutex.Unlock()

	p.seriesMutex.Lock()
	for _, key := range staleSeries {
		delete(p.reqTotal, key)
		delete(p.reqFailedTotal, key)
		delete(p.reqDurationSecondsCount, key)
		delete(p.reqDurationSecondsSum, key)
		delete(p.reqDurationSecondsBucketCounts, key)
	}
	p.seriesMutex.Unlock()
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
