// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package intervalprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor/internal/metrics"
)

var _ processor.Metrics = (*intervalProcessor)(nil)

// histogramAggregationState holds the collected values for a single stream that will be aggregated into a histogram.
type histogramAggregationState struct {
	// values collected during the interval (for explicit histograms)
	values []float64
	// exponential histogram aggregator (for exponential histograms)
	expHistogram *structure.Histogram[float64]
	// attributes of the stream
	attrs pcommon.Map
	// resource identity
	resID identity.Resource
	// scope identity
	scopeID identity.Scope
	// original resource
	resource pcommon.Resource
	// original scope
	scope pcommon.InstrumentationScope
	// resourceSchemaUrl
	resourceSchemaUrl string
	// scopeSchemaUrl
	scopeSchemaUrl string
	// original metric name
	metricName string
	// original metric unit
	unit string
	// original metric description
	description string
	// start timestamp for the histogram
	startTimestamp pcommon.Timestamp
	// latest timestamp seen
	timestamp pcommon.Timestamp
}

type intervalProcessor struct {
	ctx    context.Context
	cancel context.CancelFunc
	logger *zap.Logger

	stateLock sync.Mutex

	md                 pmetric.Metrics
	rmLookup           map[identity.Resource]pmetric.ResourceMetrics
	smLookup           map[identity.Scope]pmetric.ScopeMetrics
	mLookup            map[identity.Metric]pmetric.Metric
	numberLookup       map[identity.Stream]pmetric.NumberDataPoint
	histogramLookup    map[identity.Stream]pmetric.HistogramDataPoint
	expHistogramLookup map[identity.Stream]pmetric.ExponentialHistogramDataPoint
	summaryLookup      map[identity.Stream]pmetric.SummaryDataPoint

	// histogramAggregationConfigs maps metric name to its histogram aggregation config
	histogramAggregationConfigs map[string]AggregateToHistogram
	// histogramAggregationLookup maps stream ID to collected values for histogram aggregation
	histogramAggregationLookup map[identity.Stream]*histogramAggregationState

	config *Config

	nextConsumer consumer.Metrics
}

func newProcessor(config *Config, log *zap.Logger, nextConsumer consumer.Metrics) *intervalProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	// Build histogram aggregation config lookup
	histogramAggregationConfigs := make(map[string]AggregateToHistogram)
	for _, agg := range config.AggregateToHistogram {
		histogramAggregationConfigs[agg.MetricName] = agg
	}

	return &intervalProcessor{
		ctx:    ctx,
		cancel: cancel,
		logger: log,

		stateLock: sync.Mutex{},

		md:                 pmetric.NewMetrics(),
		rmLookup:           map[identity.Resource]pmetric.ResourceMetrics{},
		smLookup:           map[identity.Scope]pmetric.ScopeMetrics{},
		mLookup:            map[identity.Metric]pmetric.Metric{},
		numberLookup:       map[identity.Stream]pmetric.NumberDataPoint{},
		histogramLookup:    map[identity.Stream]pmetric.HistogramDataPoint{},
		expHistogramLookup: map[identity.Stream]pmetric.ExponentialHistogramDataPoint{},
		summaryLookup:      map[identity.Stream]pmetric.SummaryDataPoint{},

		histogramAggregationConfigs: histogramAggregationConfigs,
		histogramAggregationLookup:  map[identity.Stream]*histogramAggregationState{},

		config: config,

		nextConsumer: nextConsumer,
	}
}

func (p *intervalProcessor) Start(_ context.Context, _ component.Host) error {
	exportTicker := time.NewTicker(p.config.Interval)
	go func() {
		for {
			select {
			case <-p.ctx.Done():
				exportTicker.Stop()
				return
			case <-exportTicker.C:
				p.exportMetrics()
			}
		}
	}()

	return nil
}

func (p *intervalProcessor) Shutdown(_ context.Context) error {
	p.cancel()
	return nil
}

func (*intervalProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *intervalProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var errs error

	p.stateLock.Lock()
	defer p.stateLock.Unlock()

	md.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rm.ScopeMetrics().RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			sm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				// Check if this metric should be aggregated to histogram
				if _, ok := p.histogramAggregationConfigs[m.Name()]; ok {
					if m.Type() == pmetric.MetricTypeGauge || m.Type() == pmetric.MetricTypeSum {
						p.collectForHistogramAggregation(rm, sm, m)
						return true
					}
				}

				switch m.Type() {
				case pmetric.MetricTypeSummary:
					if p.config.PassThrough.Summary {
						return false
					}

					mClone, metricID := p.getOrCloneMetric(rm, sm, m)
					aggregateDataPoints(m.Summary().DataPoints(), mClone.Summary().DataPoints(), metricID, p.summaryLookup)
					return true
				case pmetric.MetricTypeGauge:
					if p.config.PassThrough.Gauge {
						return false
					}

					mClone, metricID := p.getOrCloneMetric(rm, sm, m)
					aggregateDataPoints(m.Gauge().DataPoints(), mClone.Gauge().DataPoints(), metricID, p.numberLookup)
					return true
				case pmetric.MetricTypeSum:
					// Check if we care about this value
					sum := m.Sum()

					if !sum.IsMonotonic() {
						return false
					}

					if sum.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return false
					}

					mClone, metricID := p.getOrCloneMetric(rm, sm, m)
					cloneSum := mClone.Sum()

					aggregateDataPoints(sum.DataPoints(), cloneSum.DataPoints(), metricID, p.numberLookup)
					return true
				case pmetric.MetricTypeHistogram:
					histogram := m.Histogram()

					if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return false
					}

					mClone, metricID := p.getOrCloneMetric(rm, sm, m)
					cloneHistogram := mClone.Histogram()

					aggregateDataPoints(histogram.DataPoints(), cloneHistogram.DataPoints(), metricID, p.histogramLookup)
					return true
				case pmetric.MetricTypeExponentialHistogram:
					expHistogram := m.ExponentialHistogram()

					if expHistogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return false
					}

					mClone, metricID := p.getOrCloneMetric(rm, sm, m)
					cloneExpHistogram := mClone.ExponentialHistogram()

					aggregateDataPoints(expHistogram.DataPoints(), cloneExpHistogram.DataPoints(), metricID, p.expHistogramLookup)
					return true
				default:
					errs = errors.Join(fmt.Errorf("invalid MetricType %d", m.Type()))
					return false
				}
			})
			return sm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})

	if err := p.nextConsumer.ConsumeMetrics(ctx, md); err != nil {
		errs = errors.Join(errs, err)
	}

	return errs
}

func aggregateDataPoints[DPS metrics.DataPointSlice[DP], DP metrics.DataPoint[DP]](dataPoints, mCloneDataPoints DPS, metricID identity.Metric, dpLookup map[identity.Stream]DP) {
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		streamID := identity.OfStream(metricID, dp)
		existingDP, ok := dpLookup[streamID]
		if !ok {
			dpClone := mCloneDataPoints.AppendEmpty()
			dp.CopyTo(dpClone)
			dpLookup[streamID] = dpClone
			continue
		}

		// Check if the datapoint is newer
		if dp.Timestamp() > existingDP.Timestamp() {
			dp.CopyTo(existingDP)
			continue
		}

		// Otherwise, we leave existing as-is
	}
}

func (p *intervalProcessor) exportMetrics() {
	md := func() pmetric.Metrics {
		p.stateLock.Lock()
		defer p.stateLock.Unlock()

		// ConsumeMetrics() has prepared our own pmetric.Metrics instance ready for us to use
		// Take it and clear replace it with a new empty one
		out := p.md
		p.md = pmetric.NewMetrics()

		// Build histograms from collected values
		p.buildHistograms(out)

		// Clear all the lookup references
		clear(p.rmLookup)
		clear(p.smLookup)
		clear(p.mLookup)
		clear(p.numberLookup)
		clear(p.histogramLookup)
		clear(p.expHistogramLookup)
		clear(p.summaryLookup)
		clear(p.histogramAggregationLookup)

		return out
	}()

	if err := p.nextConsumer.ConsumeMetrics(p.ctx, md); err != nil {
		p.logger.Error("Metrics export failed", zap.Error(err))
	}
}

func (p *intervalProcessor) getOrCloneMetric(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric) (pmetric.Metric, identity.Metric) {
	// Find the ResourceMetrics
	resID := identity.OfResource(rm.Resource())
	rmClone, ok := p.rmLookup[resID]
	if !ok {
		// We need to clone it *without* the ScopeMetricsSlice data
		rmClone = p.md.ResourceMetrics().AppendEmpty()
		rm.Resource().CopyTo(rmClone.Resource())
		rmClone.SetSchemaUrl(rm.SchemaUrl())
		p.rmLookup[resID] = rmClone
	}

	// Find the ScopeMetrics
	scopeID := identity.OfScope(resID, sm.Scope())
	smClone, ok := p.smLookup[scopeID]
	if !ok {
		// We need to clone it *without* the MetricSlice data
		smClone = rmClone.ScopeMetrics().AppendEmpty()
		sm.Scope().CopyTo(smClone.Scope())
		smClone.SetSchemaUrl(sm.SchemaUrl())
		p.smLookup[scopeID] = smClone
	}

	// Find the Metric
	metricID := identity.OfMetric(scopeID, m)
	mClone, ok := p.mLookup[metricID]
	if !ok {
		// We need to clone it *without* the datapoint data
		mClone = smClone.Metrics().AppendEmpty()
		mClone.SetName(m.Name())
		mClone.SetDescription(m.Description())
		mClone.SetUnit(m.Unit())

		switch m.Type() {
		case pmetric.MetricTypeGauge:
			mClone.SetEmptyGauge()
		case pmetric.MetricTypeSummary:
			mClone.SetEmptySummary()
		case pmetric.MetricTypeSum:
			src := m.Sum()

			dest := mClone.SetEmptySum()
			dest.SetAggregationTemporality(src.AggregationTemporality())
			dest.SetIsMonotonic(src.IsMonotonic())
		case pmetric.MetricTypeHistogram:
			src := m.Histogram()

			dest := mClone.SetEmptyHistogram()
			dest.SetAggregationTemporality(src.AggregationTemporality())
		case pmetric.MetricTypeExponentialHistogram:
			src := m.ExponentialHistogram()

			dest := mClone.SetEmptyExponentialHistogram()
			dest.SetAggregationTemporality(src.AggregationTemporality())
		}

		p.mLookup[metricID] = mClone
	}

	return mClone, metricID
}

// collectForHistogramAggregation collects values from gauge or counter metrics for histogram aggregation.
func (p *intervalProcessor) collectForHistogramAggregation(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric) {
	resID := identity.OfResource(rm.Resource())
	scopeID := identity.OfScope(resID, sm.Scope())
	metricID := identity.OfMetric(scopeID, m)

	var dataPoints pmetric.NumberDataPointSlice
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dataPoints = m.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dataPoints = m.Sum().DataPoints()
	default:
		return
	}

	// Get the histogram config to determine if we need exponential histogram
	aggConfig := p.histogramAggregationConfigs[m.Name()]
	isExponential := aggConfig.HistogramType == HistogramTypeExponential

	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)
		streamID := identity.OfStream(metricID, dp)

		state, ok := p.histogramAggregationLookup[streamID]
		if !ok {
			state = &histogramAggregationState{
				attrs:             pcommon.NewMap(),
				resID:             resID,
				scopeID:           scopeID,
				resource:          pcommon.NewResource(),
				scope:             pcommon.NewInstrumentationScope(),
				resourceSchemaUrl: rm.SchemaUrl(),
				scopeSchemaUrl:    sm.SchemaUrl(),
				metricName:        m.Name(),
				unit:              m.Unit(),
				description:       m.Description(),
				startTimestamp:    dp.Timestamp(),
				timestamp:         dp.Timestamp(),
			}

			if isExponential {
				// Initialize exponential histogram
				maxSize := aggConfig.MaxSize
				if maxSize <= 0 {
					maxSize = DefaultExponentialHistogramMaxSize
				}
				state.expHistogram = structure.NewFloat64(
					structure.NewConfig(structure.WithMaxSize(maxSize)),
				)
			} else {
				// Initialize explicit histogram value collection
				state.values = make([]float64, 0)
			}

			dp.Attributes().CopyTo(state.attrs)
			rm.Resource().CopyTo(state.resource)
			sm.Scope().CopyTo(state.scope)
			p.histogramAggregationLookup[streamID] = state
		}

		// Extract the value
		var value float64
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleValue()
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntValue())
		default:
			continue
		}

		if isExponential {
			state.expHistogram.Update(value)
		} else {
			state.values = append(state.values, value)
		}

		// Update timestamp to the latest seen
		if dp.Timestamp() > state.timestamp {
			state.timestamp = dp.Timestamp()
		}
		// Update start timestamp to the earliest seen
		if dp.Timestamp() < state.startTimestamp {
			state.startTimestamp = dp.Timestamp()
		}
	}
}

// buildHistograms converts collected values into histogram metrics and adds them to the output.
func (p *intervalProcessor) buildHistograms(out pmetric.Metrics) {
	if len(p.histogramAggregationLookup) == 0 {
		return
	}

	// Group by resource -> scope -> metric name -> streams
	type scopeStreams struct {
		scope          pcommon.InstrumentationScope
		scopeSchemaUrl string
		metrics        map[string]map[identity.Stream]*histogramAggregationState
	}
	type resourceStreams struct {
		resource          pcommon.Resource
		resourceSchemaUrl string
		scopes            map[identity.Scope]*scopeStreams
	}

	resourceMap := make(map[identity.Resource]*resourceStreams)

	for streamID, state := range p.histogramAggregationLookup {
		// Get or create resource entry
		rs, ok := resourceMap[state.resID]
		if !ok {
			rs = &resourceStreams{
				resource:          state.resource,
				resourceSchemaUrl: state.resourceSchemaUrl,
				scopes:            make(map[identity.Scope]*scopeStreams),
			}
			resourceMap[state.resID] = rs
		}

		// Get or create scope entry
		ss, ok := rs.scopes[state.scopeID]
		if !ok {
			ss = &scopeStreams{
				scope:          state.scope,
				scopeSchemaUrl: state.scopeSchemaUrl,
				metrics:        make(map[string]map[identity.Stream]*histogramAggregationState),
			}
			rs.scopes[state.scopeID] = ss
		}

		// Use the metric name stored in the state
		metricName := state.metricName
		if _, exists := ss.metrics[metricName]; !exists {
			ss.metrics[metricName] = make(map[identity.Stream]*histogramAggregationState)
		}
		ss.metrics[metricName][streamID] = state
	}

	// Build the output metrics
	for _, rs := range resourceMap {
		rm := out.ResourceMetrics().AppendEmpty()
		rs.resource.CopyTo(rm.Resource())
		rm.SetSchemaUrl(rs.resourceSchemaUrl)

		for _, ss := range rs.scopes {
			sm := rm.ScopeMetrics().AppendEmpty()
			ss.scope.CopyTo(sm.Scope())
			sm.SetSchemaUrl(ss.scopeSchemaUrl)

			for metricName, streams := range ss.metrics {
				aggConfig := p.histogramAggregationConfigs[metricName]
				isExponential := aggConfig.HistogramType == HistogramTypeExponential

				m := sm.Metrics().AppendEmpty()
				outputName := aggConfig.OutputName
				if outputName == "" {
					outputName = metricName + "_histogram"
				}
				m.SetName(outputName)

				// Get description and unit from first state
				for _, state := range streams {
					m.SetDescription(state.description)
					m.SetUnit(state.unit)
					break
				}

				if isExponential {
					p.buildExponentialHistogramDataPoints(m, streams)
				} else {
					p.buildExplicitHistogramDataPoints(m, streams, aggConfig.Buckets)
				}
			}
		}
	}
}

// buildExplicitHistogramDataPoints creates explicit bucket histogram data points from collected values.
func (p *intervalProcessor) buildExplicitHistogramDataPoints(m pmetric.Metric, streams map[identity.Stream]*histogramAggregationState, buckets []float64) {
	hist := m.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	for _, state := range streams {
		if len(state.values) == 0 {
			continue
		}

		dp := hist.DataPoints().AppendEmpty()
		state.attrs.CopyTo(dp.Attributes())
		dp.SetStartTimestamp(state.startTimestamp)
		dp.SetTimestamp(state.timestamp)

		// Calculate histogram from values
		bucketCounts := make([]uint64, len(buckets)+1)
		var sum float64
		var min, max float64
		count := uint64(len(state.values))

		if count > 0 {
			min = state.values[0]
			max = state.values[0]
		}

		for _, v := range state.values {
			sum += v
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}

			// Find the bucket for this value
			// Histogram bucket convention: bucket[i] counts values <= explicit_bounds[i]
			// The last bucket (overflow) counts values > explicit_bounds[n-1]
			// sort.SearchFloat64s returns the smallest index i where buckets[i] >= v
			bucketIdx := sort.SearchFloat64s(buckets, v)
			// No adjustment needed: if v <= buckets[bucketIdx], it goes in bucket[bucketIdx]
			// If v > all buckets (bucketIdx == len(buckets)), it goes in the overflow bucket
			bucketCounts[bucketIdx]++
		}

		dp.SetCount(count)
		dp.SetSum(sum)
		dp.SetMin(min)
		dp.SetMax(max)
		dp.ExplicitBounds().FromRaw(buckets)
		dp.BucketCounts().FromRaw(bucketCounts)
	}
}

// buildExponentialHistogramDataPoints creates exponential histogram data points from the aggregated exponential histogram.
func (p *intervalProcessor) buildExponentialHistogramDataPoints(m pmetric.Metric, streams map[identity.Stream]*histogramAggregationState) {
	expHist := m.SetEmptyExponentialHistogram()
	expHist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	for _, state := range streams {
		if state.expHistogram == nil || state.expHistogram.Count() == 0 {
			continue
		}

		dp := expHist.DataPoints().AppendEmpty()
		state.attrs.CopyTo(dp.Attributes())
		dp.SetStartTimestamp(state.startTimestamp)
		dp.SetTimestamp(state.timestamp)

		// Copy data from the exponential histogram aggregator
		dp.SetCount(state.expHistogram.Count())
		dp.SetSum(state.expHistogram.Sum())
		dp.SetScale(state.expHistogram.Scale())
		dp.SetZeroCount(state.expHistogram.ZeroCount())

		if state.expHistogram.Count() > 0 {
			dp.SetMin(state.expHistogram.Min())
			dp.SetMax(state.expHistogram.Max())
		}

		// Copy positive buckets
		copyExponentialBuckets(state.expHistogram.Positive(), dp.Positive())
		// Copy negative buckets
		copyExponentialBuckets(state.expHistogram.Negative(), dp.Negative())
	}
}

// copyExponentialBuckets copies bucket data from the go-expohisto structure to the pmetric structure.
func copyExponentialBuckets(src *structure.Buckets, dest pmetric.ExponentialHistogramDataPointBuckets) {
	dest.SetOffset(src.Offset())
	dest.BucketCounts().EnsureCapacity(int(src.Len()))
	for i := uint32(0); i < src.Len(); i++ {
		dest.BucketCounts().Append(src.At(i))
	}
}
