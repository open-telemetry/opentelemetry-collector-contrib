// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package intervalprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor/internal/metrics"
)

var _ processor.Metrics = (*Processor)(nil)

type Processor struct {
	ctx    context.Context
	cancel context.CancelFunc
	logger *zap.Logger

	stateLock sync.Mutex

	partitions    []*Partition
	numPartitions int // Store number of partitions to avoid len(partitions) calls all the time.
	config        *Config

	nextConsumer consumer.Metrics
}

type Partition struct {
	md                 pmetric.Metrics
	rmLookup           map[identity.Resource]pmetric.ResourceMetrics
	smLookup           map[identity.Scope]pmetric.ScopeMetrics
	mLookup            map[identity.Metric]pmetric.Metric
	numberLookup       map[identity.Stream]pmetric.NumberDataPoint
	histogramLookup    map[identity.Stream]pmetric.HistogramDataPoint
	expHistogramLookup map[identity.Stream]pmetric.ExponentialHistogramDataPoint
	summaryLookup      map[identity.Stream]pmetric.SummaryDataPoint
}

func newProcessor(config *Config, log *zap.Logger, nextConsumer consumer.Metrics) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	numPartitions := int(config.Interval.Seconds())

	partitions := make([]*Partition, numPartitions)
	for i := range partitions {
		partitions[i] = &Partition{
			md:                 pmetric.NewMetrics(),
			rmLookup:           make(map[identity.Resource]pmetric.ResourceMetrics, 0),
			smLookup:           make(map[identity.Scope]pmetric.ScopeMetrics, 0),
			mLookup:            make(map[identity.Metric]pmetric.Metric, 0),
			numberLookup:       make(map[identity.Stream]pmetric.NumberDataPoint, 0),
			histogramLookup:    make(map[identity.Stream]pmetric.HistogramDataPoint, 0),
			expHistogramLookup: make(map[identity.Stream]pmetric.ExponentialHistogramDataPoint, 0),
			summaryLookup:      make(map[identity.Stream]pmetric.SummaryDataPoint, 0),
		}
	}

	return &Processor{
		ctx:           ctx,
		cancel:        cancel,
		logger:        log,
		stateLock:     sync.Mutex{},
		partitions:    partitions,
		numPartitions: numPartitions,
		config:        config,

		nextConsumer: nextConsumer,
	}
}

func (p *Processor) Start(_ context.Context, _ component.Host) error {
	exportTicker := time.NewTicker(time.Second)
	i := 0
	go func() {
		for {
			select {
			case <-p.ctx.Done():
				exportTicker.Stop()
				return
			case <-exportTicker.C:
				p.exportMetrics(i)
				i = (i + 1) % p.numPartitions
			}
		}
	}()

	return nil
}

func (p *Processor) Shutdown(_ context.Context) error {
	p.cancel()
	return nil
}

func (p *Processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *Processor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var errs error

	p.stateLock.Lock()
	defer p.stateLock.Unlock()

	md.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rm.ScopeMetrics().RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			sm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				switch m.Type() {
				case pmetric.MetricTypeSummary:
					if p.config.PassThrough.Summary {
						return false
					}

					mClone, metricID, partition := p.getOrCloneMetric(rm, sm, m)
					aggregateDataPoints(m.Summary().DataPoints(), mClone.Summary().DataPoints(), metricID, p.partitions[partition].summaryLookup)
					return true
				case pmetric.MetricTypeGauge:
					if p.config.PassThrough.Gauge {
						return false
					}

					mClone, metricID, partition := p.getOrCloneMetric(rm, sm, m)
					aggregateDataPoints(m.Gauge().DataPoints(), mClone.Gauge().DataPoints(), metricID, p.partitions[partition].numberLookup)
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

					mClone, metricID, partition := p.getOrCloneMetric(rm, sm, m)
					cloneSum := mClone.Sum()

					aggregateDataPoints(sum.DataPoints(), cloneSum.DataPoints(), metricID, p.partitions[partition].numberLookup)
					return true
				case pmetric.MetricTypeHistogram:
					histogram := m.Histogram()

					if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return false
					}

					mClone, metricID, partition := p.getOrCloneMetric(rm, sm, m)
					cloneHistogram := mClone.Histogram()

					aggregateDataPoints(histogram.DataPoints(), cloneHistogram.DataPoints(), metricID, p.partitions[partition].histogramLookup)
					return true
				case pmetric.MetricTypeExponentialHistogram:
					expHistogram := m.ExponentialHistogram()

					if expHistogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return false
					}

					mClone, metricID, partition := p.getOrCloneMetric(rm, sm, m)
					cloneExpHistogram := mClone.ExponentialHistogram()

					aggregateDataPoints(expHistogram.DataPoints(), cloneExpHistogram.DataPoints(), metricID, p.partitions[partition].expHistogramLookup)
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

func aggregateDataPoints[DPS metrics.DataPointSlice[DP], DP metrics.DataPoint[DP]](dataPoints DPS, mCloneDataPoints DPS, metricID identity.Metric, dpLookup map[identity.Stream]DP) {
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

func (p *Processor) exportMetrics(partition int) {
	md := func() pmetric.Metrics {
		p.stateLock.Lock()
		defer p.stateLock.Unlock()

		// ConsumeMetrics() has prepared our own pmetric.Metrics instance ready for us to use
		// Take it and clear replace it with a new empty one
		out := p.partitions[partition].md
		p.partitions[partition].md = pmetric.NewMetrics()

		// Clear all the lookup references
		clear(p.partitions[partition].rmLookup)
		clear(p.partitions[partition].smLookup)
		clear(p.partitions[partition].mLookup)
		clear(p.partitions[partition].numberLookup)
		clear(p.partitions[partition].histogramLookup)
		clear(p.partitions[partition].expHistogramLookup)
		clear(p.partitions[partition].summaryLookup)

		return out
	}()

	if err := p.nextConsumer.ConsumeMetrics(p.ctx, md); err != nil {
		p.logger.Error("Metrics export failed", zap.Error(err))
	}
}

func (p *Processor) getOrCloneMetric(rm pmetric.ResourceMetrics, sm pmetric.ScopeMetrics, m pmetric.Metric) (pmetric.Metric, identity.Metric, uint64) {
	// Find the ResourceMetrics
	resID := identity.OfResource(rm.Resource())
	scopeID := identity.OfScope(resID, sm.Scope())
	metricID := identity.OfMetric(scopeID, m)
	var mClone pmetric.Metric
	var ok bool
	for i, partition := range p.partitions {
		mClone, ok = partition.mLookup[metricID]
		if ok {
			// int -> uint64 theoretically can lead to overflow.
			// The only way we get an overflow here is if the interval is greater than 18446744073709551615 seconds.
			// That's 584942417355 years. I think we're safe.
			return mClone, metricID, uint64(i) //nolint
		}
	}

	var rmClone pmetric.ResourceMetrics
	var smClone pmetric.ScopeMetrics
	// Getting here means the metric isn't stored in any partition, so we need to create it.

	// int -> uint64 theoretically can lead to overflow.
	// The only way we get an overflow here is if the interval is greater than 18446744073709551615 seconds.
	// That's 584942417355 years. I think we're safe.
	partition := metricID.Hash().Sum64() % uint64(p.numPartitions) //nolint

	// We need to clone resourceMetrics *without* the ScopeMetricsSlice data
	rmClone = p.partitions[partition].md.ResourceMetrics().AppendEmpty()
	rm.Resource().CopyTo(rmClone.Resource())
	rmClone.SetSchemaUrl(rm.SchemaUrl())

	// We need to clone scopeMetrics *without* the Metric
	smClone = rmClone.ScopeMetrics().AppendEmpty()
	sm.Scope().CopyTo(smClone.Scope())
	smClone.SetSchemaUrl(sm.SchemaUrl())

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

	p.partitions[partition].rmLookup[resID] = rmClone
	p.partitions[partition].smLookup[scopeID] = smClone
	p.partitions[partition].mLookup[metricID] = mClone

	return mClone, metricID, partition
}
