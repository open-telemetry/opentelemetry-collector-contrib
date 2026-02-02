// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	conventionsv125 "go.opentelemetry.io/otel/semconv/v1.25.0"
	conventionsv128 "go.opentelemetry.io/otel/semconv/v1.28.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector/internal/store"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
)

const (
	metricKeySeparator = string(byte(0))
	clientKind         = "client"
	serverKind         = "server"
	virtualNodeLabel   = "virtual_node"
	millisecondsUnit   = "ms"
	secondsUnit        = "s"
)

var (
	legacyDefaultLatencyHistogramBuckets = []float64{
		2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000,
	}
	defaultLatencyHistogramBuckets = []float64{
		0.002, 0.004, 0.006, 0.008, 0.01, 0.05, 0.1, 0.2, 0.4, 0.8, 1, 1.4, 2, 5, 10, 15,
	}

	defaultPeerAttributes = []string{
		string(conventions.PeerServiceKey), string(conventionsv125.DBNameKey), string(conventionsv128.DBSystemKey),
	}

	defaultDatabaseNameAttributes = []string{string(conventionsv125.DBNameKey)}

	defaultMetricsFlushInterval = 60 * time.Second // 1 DPM
)

type metricSeries struct {
	dimensions  pcommon.Map
	lastUpdated int64 // Used to remove stale series
}

var _ processor.Traces = (*serviceGraphConnector)(nil)

type serviceGraphConnector struct {
	config          *Config
	logger          *zap.Logger
	metricsConsumer consumer.Metrics

	store *store.Store

	startTime time.Time

	seriesMutex                          sync.Mutex
	reqTotal                             map[string]int64
	reqFailedTotal                       map[string]int64
	reqClientDurationSecondsCount        map[string]uint64
	reqClientDurationSecondsSum          map[string]float64
	reqClientDurationSecondsBucketCounts map[string][]uint64
	reqClientDurationExpHistogram        map[string]*structure.Histogram[float64]
	reqServerDurationSecondsCount        map[string]uint64
	reqServerDurationSecondsSum          map[string]float64
	reqServerDurationSecondsBucketCounts map[string][]uint64
	reqServerDurationExpHistogram        map[string]*structure.Histogram[float64]
	reqDurationBounds                    []float64

	metricMutex sync.RWMutex
	keyToMetric map[string]metricSeries

	telemetryBuilder *metadata.TelemetryBuilder

	shutdownCh chan any
}

func newConnector(set component.TelemetrySettings, config component.Config, next consumer.Metrics) (*serviceGraphConnector, error) {
	pConfig := config.(*Config)

	var bounds []float64
	if pConfig.ExponentialHistogramMaxSize == 0 {
		bounds = defaultLatencyHistogramBuckets
		if legacyLatencyUnitMsFeatureGate.IsEnabled() {
			bounds = legacyDefaultLatencyHistogramBuckets
		}
		if pConfig.LatencyHistogramBuckets != nil {
			bounds = mapDurationsToFloat(pConfig.LatencyHistogramBuckets)
		}
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

	if len(pConfig.DatabaseNameAttributes) == 0 {
		pConfig.DatabaseNameAttributes = defaultDatabaseNameAttributes
	}

	if pConfig.MetricsFlushInterval == nil {
		pConfig.MetricsFlushInterval = &defaultMetricsFlushInterval
	} else if pConfig.MetricsFlushInterval.Nanoseconds() <= 0 {
		set.Logger.Warn("MetricsFlushInterval is set to 0, metrics will be flushed on every received batch of traces")
	}

	telemetryBuilder, err := metadata.NewTelemetryBuilder(set)
	if err != nil {
		return nil, err
	}

	return &serviceGraphConnector{
		config:          pConfig,
		logger:          set.Logger,
		metricsConsumer: next,

		startTime:                            time.Now(),
		reqTotal:                             make(map[string]int64),
		reqFailedTotal:                       make(map[string]int64),
		reqClientDurationSecondsCount:        make(map[string]uint64),
		reqClientDurationSecondsSum:          make(map[string]float64),
		reqClientDurationSecondsBucketCounts: make(map[string][]uint64),
		reqClientDurationExpHistogram:        make(map[string]*structure.Histogram[float64]),
		reqServerDurationSecondsCount:        make(map[string]uint64),
		reqServerDurationSecondsSum:          make(map[string]float64),
		reqServerDurationSecondsBucketCounts: make(map[string][]uint64),
		reqServerDurationExpHistogram:        make(map[string]*structure.Histogram[float64]),
		reqDurationBounds:                    bounds,
		keyToMetric:                          make(map[string]metricSeries),
		shutdownCh:                           make(chan any),
		telemetryBuilder:                     telemetryBuilder,
	}, nil
}

func (p *serviceGraphConnector) Start(context.Context, component.Host) error {
	p.store = store.NewStore(p.config.Store.TTL, p.config.Store.MaxItems, p.onComplete, p.onExpire)

	go p.metricFlushLoop(*p.config.MetricsFlushInterval)

	go p.cacheLoop(p.config.CacheLoop)

	go p.storeExpirationLoop(p.config.StoreExpirationLoop)

	p.logger.Info("Started servicegraphconnector")
	return nil
}

func (p *serviceGraphConnector) metricFlushLoop(flushInterval time.Duration) {
	if flushInterval <= 0 {
		return
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := p.flushMetrics(context.Background()); err != nil {
				p.logger.Error("failed to flush metrics", zap.Error(err))
			}
		case <-p.shutdownCh:
			return
		}
	}
}

func (p *serviceGraphConnector) flushMetrics(ctx context.Context) error {
	md, err := p.buildMetrics()
	if err != nil {
		return fmt.Errorf("failed to build metrics: %w", err)
	}

	// Skip empty metrics.
	if md.MetricCount() == 0 {
		return nil
	}

	// Firstly, export md to avoid being impacted by downstream trace serviceGraphConnector errors/latency.
	return p.metricsConsumer.ConsumeMetrics(ctx, md)
}

func (p *serviceGraphConnector) Shutdown(context.Context) error {
	p.logger.Info("Shutting down servicegraphconnector")
	close(p.shutdownCh)
	return nil
}

func (*serviceGraphConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (p *serviceGraphConnector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if err := p.aggregateMetrics(ctx, td); err != nil {
		return fmt.Errorf("failed to aggregate metrics: %w", err)
	}

	// If metricsFlushInterval is not set, flush metrics immediately.
	if *p.config.MetricsFlushInterval <= 0 {
		if err := p.flushMetrics(ctx); err != nil {
			// Not return error here to avoid impacting traces.
			p.logger.Error("failed to flush metrics", zap.Error(err))
		}
	}

	return nil
}

func (p *serviceGraphConnector) aggregateMetrics(ctx context.Context, td ptrace.Traces) (err error) {
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
						e.ClientLatencySec = spanDuration(span)
						e.Failed = e.Failed || span.Status().Code() == ptrace.StatusCodeError
						p.upsertDimensions(clientKind, e.Dimensions, rAttributes, span.Attributes())

						if virtualNodeFeatureGate.IsEnabled() {
							p.upsertPeerAttributes(p.config.VirtualNodePeerAttributes, e.Peer, span.Attributes())
						}

						// A database request will only have one span, we don't wait for the server
						// span but just copy details from the client span
						if dbName, ok := getFirstMatchingValue(p.config.DatabaseNameAttributes, rAttributes, span.Attributes()); ok {
							e.ConnectionType = store.Database
							e.ServerService = dbName
							e.ServerLatencySec = spanDuration(span)
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
						e.ServerLatencySec = spanDuration(span)
						e.Failed = e.Failed || span.Status().Code() == ptrace.StatusCodeError
						p.upsertDimensions(serverKind, e.Dimensions, rAttributes, span.Attributes())
					})
				default:
					// this span is not part of an edge
					continue
				}

				if errors.Is(err, store.ErrTooManyItems) {
					totalDroppedSpans++
					p.telemetryBuilder.ConnectorServicegraphDroppedSpans.Add(ctx, 1)
					continue
				}

				// UpsertEdge will only return ErrTooManyItems
				if err != nil {
					return err
				}

				if isNew {
					p.telemetryBuilder.ConnectorServicegraphTotalEdges.Add(ctx, 1)
				}
			}
		}
	}
	return nil
}

func (p *serviceGraphConnector) upsertDimensions(kind string, m map[string]string, resourceAttr, spanAttr pcommon.Map) {
	for _, dim := range p.config.Dimensions {
		if v, ok := pdatautil.GetAttributeValue(dim, resourceAttr, spanAttr); ok {
			m[kind+"_"+dim] = v
		}
	}
}

func (*serviceGraphConnector) upsertPeerAttributes(m []string, peers map[string]string, spanAttr pcommon.Map) {
	for _, s := range m {
		if v, ok := pdatautil.GetAttributeValue(s, spanAttr); ok {
			peers[s] = v
			break
		}
	}
}

func (p *serviceGraphConnector) onComplete(e *store.Edge) {
	p.logger.Debug(
		"edge completed",
		zap.String("client_service", e.ClientService),
		zap.String("server_service", e.ServerService),
		zap.String("connection_type", string(e.ConnectionType)),
		zap.Stringer("trace_id", e.TraceID),
	)
	p.aggregateMetricsForEdge(e)
}

func (p *serviceGraphConnector) onExpire(e *store.Edge) {
	p.logger.Debug(
		"edge expired",
		zap.String("client_service", e.ClientService),
		zap.String("server_service", e.ServerService),
		zap.String("connection_type", string(e.ConnectionType)),
		zap.Stringer("trace_id", e.TraceID),
	)

	p.telemetryBuilder.ConnectorServicegraphExpiredEdges.Add(context.Background(), 1)

	if virtualNodeFeatureGate.IsEnabled() && len(p.config.VirtualNodePeerAttributes) > 0 {
		e.ConnectionType = store.VirtualNode
		if e.ClientService == "" && e.Key.SpanIDIsEmpty() {
			e.ClientService = "user"
			if p.config.VirtualNodeExtraLabel {
				e.VirtualNodeLabel = store.ClientVirtualNode
			}
			p.onComplete(e)
		}

		if e.ServerService == "" {
			e.ServerService = p.getPeerHost(p.config.VirtualNodePeerAttributes, e.Peer)
			if p.config.VirtualNodeExtraLabel {
				e.VirtualNodeLabel = store.ServerVirtualNode
			}
			p.onComplete(e)
		}
	}
}

func (p *serviceGraphConnector) aggregateMetricsForEdge(e *store.Edge) {
	metricKey := p.buildMetricKey(e.ClientService, e.ServerService, string(e.ConnectionType), strconv.FormatBool(e.Failed), e.Dimensions)
	dimensions := buildDimensions(e)

	if p.config.VirtualNodeExtraLabel {
		dimensions = addExtraLabel(dimensions, virtualNodeLabel, string(e.VirtualNodeLabel))
	}

	p.seriesMutex.Lock()
	defer p.seriesMutex.Unlock()
	p.updateSeries(metricKey, dimensions)
	p.updateCountMetrics(metricKey)
	if e.Failed {
		p.updateErrorMetrics(metricKey)
	}
	p.updateDurationMetrics(metricKey, e.ServerLatencySec, e.ClientLatencySec)
}

func (p *serviceGraphConnector) updateSeries(key string, dimensions pcommon.Map) {
	p.metricMutex.Lock()
	defer p.metricMutex.Unlock()
	// Overwrite the series if it already exists
	p.keyToMetric[key] = metricSeries{
		dimensions:  dimensions,
		lastUpdated: time.Now().UnixMilli(),
	}
}

func (p *serviceGraphConnector) dimensionsForSeries(key string) (pcommon.Map, bool) {
	p.metricMutex.RLock()
	defer p.metricMutex.RUnlock()
	if series, ok := p.keyToMetric[key]; ok {
		return series.dimensions, true
	}

	return pcommon.Map{}, false
}

func (p *serviceGraphConnector) updateCountMetrics(key string) { p.reqTotal[key]++ }

func (p *serviceGraphConnector) updateErrorMetrics(key string) { p.reqFailedTotal[key]++ }

func (p *serviceGraphConnector) updateDurationMetrics(key string, serverDuration, clientDuration float64) {
	p.updateServerDurationMetrics(key, serverDuration)
	p.updateClientDurationMetrics(key, clientDuration)
}

func (p *serviceGraphConnector) updateServerDurationMetrics(key string, duration float64) {
	if p.reqDurationBounds == nil {
		histogram, ok := p.reqServerDurationExpHistogram[key]
		if !ok {
			histogram = new(structure.Histogram[float64])
			cfg := structure.NewConfig(
				structure.WithMaxSize(p.config.ExponentialHistogramMaxSize),
			)
			histogram.Init(cfg)
			p.reqServerDurationExpHistogram[key] = histogram
		}

		histogram.Update(duration)
	} else {
		index := sort.SearchFloat64s(p.reqDurationBounds, duration) // Search bucket index
		if _, ok := p.reqServerDurationSecondsBucketCounts[key]; !ok {
			p.reqServerDurationSecondsBucketCounts[key] = make([]uint64, len(p.reqDurationBounds)+1)
		}

		p.reqServerDurationSecondsSum[key] += duration
		p.reqServerDurationSecondsCount[key]++
		p.reqServerDurationSecondsBucketCounts[key][index]++
	}
}

func (p *serviceGraphConnector) updateClientDurationMetrics(key string, duration float64) {
	if p.reqDurationBounds == nil {
		histogram, ok := p.reqClientDurationExpHistogram[key]
		if !ok {
			histogram = new(structure.Histogram[float64])
			cfg := structure.NewConfig(
				structure.WithMaxSize(p.config.ExponentialHistogramMaxSize),
			)
			histogram.Init(cfg)
			p.reqClientDurationExpHistogram[key] = histogram
		}

		histogram.Update(duration)
	} else {
		index := sort.SearchFloat64s(p.reqDurationBounds, duration) // Search bucket index
		if _, ok := p.reqClientDurationSecondsBucketCounts[key]; !ok {
			p.reqClientDurationSecondsBucketCounts[key] = make([]uint64, len(p.reqDurationBounds)+1)
		}

		p.reqClientDurationSecondsSum[key] += duration
		p.reqClientDurationSecondsCount[key]++
		p.reqClientDurationSecondsBucketCounts[key][index]++
	}
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

func addExtraLabel(dimensions pcommon.Map, label, value string) pcommon.Map {
	dimensions.PutStr(label, value)
	return dimensions
}

// nowWithOffset returns the current time minus the configured offset
func (p *serviceGraphConnector) nowWithOffset() time.Time {
	return time.Now().Add(-p.config.MetricsTimestampOffset)
}

func (p *serviceGraphConnector) buildMetrics() (pmetric.Metrics, error) {
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

func (p *serviceGraphConnector) collectCountMetrics(ilm pmetric.ScopeMetrics) error {
	if len(p.reqTotal) > 0 {
		mCount := ilm.Metrics().AppendEmpty()
		mCount.SetName("traces_service_graph_request_total")
		mCount.SetEmptySum().SetIsMonotonic(true)
		// TODO: Support other aggregation temporalities
		mCount.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		for key, c := range p.reqTotal {
			dpCalls := mCount.Sum().DataPoints().AppendEmpty()
			dpCalls.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
			dpCalls.SetTimestamp(pcommon.NewTimestampFromTime(p.nowWithOffset()))
			dpCalls.SetIntValue(c)

			dimensions, ok := p.dimensionsForSeries(key)
			if !ok {
				return fmt.Errorf("failed to find dimensions for key %s", key)
			}

			dimensions.CopyTo(dpCalls.Attributes())
		}
	}

	if len(p.reqFailedTotal) > 0 {
		mCount := ilm.Metrics().AppendEmpty()
		mCount.SetName("traces_service_graph_request_failed_total")
		mCount.SetEmptySum().SetIsMonotonic(true)
		// TODO: Support other aggregation temporalities
		mCount.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		for key, c := range p.reqFailedTotal {
			dpCalls := mCount.Sum().DataPoints().AppendEmpty()
			dpCalls.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
			dpCalls.SetTimestamp(pcommon.NewTimestampFromTime(p.nowWithOffset()))
			dpCalls.SetIntValue(c)

			dimensions, ok := p.dimensionsForSeries(key)
			if !ok {
				return fmt.Errorf("failed to find dimensions for key %s", key)
			}

			dimensions.CopyTo(dpCalls.Attributes())
		}
	}

	return nil
}

func (p *serviceGraphConnector) collectLatencyMetrics(ilm pmetric.ScopeMetrics) error {
	// TODO: Remove this once legacy metric names are removed
	if legacyMetricNamesFeatureGate.IsEnabled() {
		return p.collectServerLatencyMetrics(ilm, "traces_service_graph_request_duration")
	}

	if err := p.collectServerLatencyMetrics(ilm, "traces_service_graph_request_server"); err != nil {
		return err
	}

	return p.collectClientLatencyMetrics(ilm)
}

func (p *serviceGraphConnector) collectClientLatencyMetrics(ilm pmetric.ScopeMetrics) error {
	mDuration := pmetric.NewMetric()
	mDuration.SetName("traces_service_graph_request_client")
	mDuration.SetUnit(secondsUnit)
	if legacyLatencyUnitMsFeatureGate.IsEnabled() {
		mDuration.SetUnit(millisecondsUnit)
	}

	if p.reqDurationBounds == nil {
		mDuration.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		for key, expHistogram := range p.reqClientDurationExpHistogram {
			dpDuration := mDuration.ExponentialHistogram().DataPoints().AppendEmpty()
			dpDuration.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
			dimensions, ok := p.dimensionsForSeries(key)
			if !ok {
				return fmt.Errorf("failed to find dimensions for key %s", key)
			}

			dimensions.CopyTo(dpDuration.Attributes())
			dpDuration.SetCount(expHistogram.Count())
			dpDuration.SetSum(expHistogram.Sum())
			pdatautil.ExpoHistToExponentialDataPoint(expHistogram, dpDuration)
		}
		mDuration.CopyTo(ilm.Metrics().AppendEmpty())
	} else if len(p.reqClientDurationSecondsCount) > 0 {
		// TODO: Support other aggregation temporalities
		mDuration.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		timestamp := pcommon.NewTimestampFromTime(p.nowWithOffset())

		for key := range p.reqClientDurationSecondsCount {
			dpDuration := mDuration.Histogram().DataPoints().AppendEmpty()
			dpDuration.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
			dpDuration.SetTimestamp(timestamp)
			dpDuration.ExplicitBounds().FromRaw(p.reqDurationBounds)
			dpDuration.BucketCounts().FromRaw(p.reqClientDurationSecondsBucketCounts[key])
			dpDuration.SetCount(p.reqClientDurationSecondsCount[key])
			dpDuration.SetSum(p.reqClientDurationSecondsSum[key])

			// TODO: Support exemplars
			dimensions, ok := p.dimensionsForSeries(key)
			if !ok {
				return fmt.Errorf("failed to find dimensions for key %s", key)
			}

			dimensions.CopyTo(dpDuration.Attributes())
		}
		mDuration.CopyTo(ilm.Metrics().AppendEmpty())
	}
	return nil
}

func (p *serviceGraphConnector) collectServerLatencyMetrics(ilm pmetric.ScopeMetrics, mName string) error {
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	mDuration := pmetric.NewMetric()
	mDuration.SetName(mName)
	mDuration.SetUnit(secondsUnit)
	if legacyLatencyUnitMsFeatureGate.IsEnabled() {
		mDuration.SetUnit(millisecondsUnit)
	}

	if p.reqDurationBounds == nil {
		mDuration.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		for key, expHistogram := range p.reqServerDurationExpHistogram {
			dpDuration := mDuration.ExponentialHistogram().DataPoints().AppendEmpty()
			dpDuration.SetTimestamp(timestamp)
			dpDuration.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
			dimensions, ok := p.dimensionsForSeries(key)
			if !ok {
				return fmt.Errorf("failed to find dimensions for key %s", key)
			}

			dimensions.CopyTo(dpDuration.Attributes())
			dpDuration.SetCount(expHistogram.Count())
			dpDuration.SetSum(expHistogram.Sum())
			pdatautil.ExpoHistToExponentialDataPoint(expHistogram, dpDuration)
		}
		mDuration.CopyTo(ilm.Metrics().AppendEmpty())
	} else if len(p.reqServerDurationSecondsCount) > 0 {
		// TODO: Support other aggregation temporalities
		mDuration.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		timestamp := pcommon.NewTimestampFromTime(p.nowWithOffset())

		for key := range p.reqServerDurationSecondsCount {
			dpDuration := mDuration.Histogram().DataPoints().AppendEmpty()
			dpDuration.SetStartTimestamp(pcommon.NewTimestampFromTime(p.startTime))
			dpDuration.SetTimestamp(timestamp)
			dpDuration.ExplicitBounds().FromRaw(p.reqDurationBounds)
			dpDuration.BucketCounts().FromRaw(p.reqServerDurationSecondsBucketCounts[key])
			dpDuration.SetCount(p.reqServerDurationSecondsCount[key])
			dpDuration.SetSum(p.reqServerDurationSecondsSum[key])

			// TODO: Support exemplars
			dimensions, ok := p.dimensionsForSeries(key)
			if !ok {
				return fmt.Errorf("failed to find dimensions for key %s", key)
			}

			dimensions.CopyTo(dpDuration.Attributes())
		}
		mDuration.CopyTo(ilm.Metrics().AppendEmpty())
	}
	return nil
}

func (p *serviceGraphConnector) buildMetricKey(clientName, serverName, connectionType, failed string, edgeDimensions map[string]string) string {
	var metricKey strings.Builder
	metricKey.WriteString(clientName + metricKeySeparator + serverName + metricKeySeparator + connectionType + metricKeySeparator + failed)

	for _, dimName := range p.config.Dimensions {
		for _, kind := range []string{clientKind, serverKind} {
			dim, ok := edgeDimensions[kind+"_"+dimName]
			if !ok {
				continue
			}
			metricKey.WriteString(metricKeySeparator + kind + "_" + dimName + "_" + dim)
		}
	}

	return metricKey.String()
}

// storeExpirationLoop periodically expires old entries from the store.
func (p *serviceGraphConnector) storeExpirationLoop(d time.Duration) {
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

func (*serviceGraphConnector) getPeerHost(m []string, peers map[string]string) string {
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
func (p *serviceGraphConnector) cacheLoop(d time.Duration) {
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
func (p *serviceGraphConnector) cleanCache() {
	var staleSeries []string
	p.metricMutex.RLock()
	for key, series := range p.keyToMetric {
		if series.lastUpdated+15*time.Minute.Milliseconds() < time.Now().UnixMilli() {
			staleSeries = append(staleSeries, key)
		}
	}
	p.metricMutex.RUnlock()

	p.seriesMutex.Lock()
	for _, key := range staleSeries {
		delete(p.reqTotal, key)
		delete(p.reqFailedTotal, key)
		delete(p.reqClientDurationSecondsCount, key)
		delete(p.reqClientDurationSecondsSum, key)
		delete(p.reqClientDurationSecondsBucketCounts, key)
		delete(p.reqServerDurationSecondsCount, key)
		delete(p.reqServerDurationSecondsSum, key)
		delete(p.reqServerDurationSecondsBucketCounts, key)
		delete(p.reqServerDurationExpHistogram, key)
		delete(p.reqClientDurationExpHistogram, key)
	}
	p.seriesMutex.Unlock()

	p.metricMutex.Lock()
	for _, key := range staleSeries {
		delete(p.keyToMetric, key)
	}
	p.metricMutex.Unlock()
}

// spanDuration returns the duration of the given span in seconds (legacy ms).
func spanDuration(span ptrace.Span) float64 {
	if legacyLatencyUnitMsFeatureGate.IsEnabled() {
		return float64(span.EndTimestamp()-span.StartTimestamp()) / float64(time.Millisecond.Nanoseconds())
	}
	return float64(span.EndTimestamp()-span.StartTimestamp()) / float64(time.Second.Nanoseconds())
}

// durationToFloat converts the given duration to the number of seconds (legacy ms) it represents.
func durationToFloat(d time.Duration) float64 {
	if legacyLatencyUnitMsFeatureGate.IsEnabled() {
		return float64(d.Milliseconds())
	}
	return d.Seconds()
}

func mapDurationsToFloat(vs []time.Duration) []float64 {
	vsm := make([]float64, len(vs))
	for i, v := range vs {
		vsm[i] = durationToFloat(v)
	}
	return vsm
}
