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
	"go.opentelemetry.io/collector/pdata/pcommon"
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

	numbers       map[identity.Stream]metrics.StreamDataPoint[pmetric.NumberDataPoint]
	histograms    map[identity.Stream]metrics.StreamDataPoint[pmetric.HistogramDataPoint]
	expHistograms map[identity.Stream]metrics.StreamDataPoint[pmetric.ExponentialHistogramDataPoint]

	exportInterval time.Duration
	exportTicker   *time.Ticker

	nextConsumer consumer.Metrics
}

func newProcessor(config *Config, log *zap.Logger, nextConsumer consumer.Metrics) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Processor{
		ctx:    ctx,
		cancel: cancel,
		logger: log,

		stateLock:     sync.Mutex{},
		numbers:       map[identity.Stream]metrics.StreamDataPoint[pmetric.NumberDataPoint]{},
		histograms:    map[identity.Stream]metrics.StreamDataPoint[pmetric.HistogramDataPoint]{},
		expHistograms: map[identity.Stream]metrics.StreamDataPoint[pmetric.ExponentialHistogramDataPoint]{},

		exportInterval: config.Interval,

		nextConsumer: nextConsumer,
	}
}

func (p *Processor) Start(_ context.Context, _ component.Host) error {
	p.exportTicker = time.NewTicker(p.exportInterval)
	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-p.exportTicker.C:
				p.exportMetrics()
			}
		}
	}()

	return nil
}

func (p *Processor) Shutdown(_ context.Context) error {
	if p.exportTicker != nil {
		p.exportTicker.Stop()
	}
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
				case pmetric.MetricTypeGauge, pmetric.MetricTypeSummary:
					return false
				case pmetric.MetricTypeSum:
					sum := m.Sum()

					if !sum.IsMonotonic() {
						return false
					}

					if sum.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return false
					}

					aggregateDataPoints(sum.DataPoints(), p.numbers, rm.Resource(), rm.SchemaUrl(), sm.Scope(), sm.SchemaUrl(), m)
					return true
				case pmetric.MetricTypeHistogram:
					histogram := m.Histogram()

					if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return false
					}

					aggregateDataPoints(histogram.DataPoints(), p.histograms, rm.Resource(), rm.SchemaUrl(), sm.Scope(), sm.SchemaUrl(), m)
					return true
				case pmetric.MetricTypeExponentialHistogram:
					expHistogram := m.ExponentialHistogram()

					if expHistogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return false
					}

					aggregateDataPoints(expHistogram.DataPoints(), p.expHistograms, rm.Resource(), rm.SchemaUrl(), sm.Scope(), sm.SchemaUrl(), m)
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

func aggregateDataPoints[DPS metrics.DataPointSlice[DP], DP metrics.DataPoint[DP]](dataPoints DPS, state map[identity.Stream]metrics.StreamDataPoint[DP], res pcommon.Resource, resSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, m pmetric.Metric) {
	metric := metrics.From(res, resSchemaURL, scope, scopeSchemaURL, m)
	metricID := metric.Identity()

	now := time.Now()

	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		streamDataPointID := metrics.StreamDataPointIdentity(metricID, dp)

		existing, ok := state[streamDataPointID]
		if !ok {
			state[streamDataPointID] = metrics.StreamDataPoint[DP]{
				Metric:      metric,
				DataPoint:   dp,
				LastUpdated: now,
			}
			continue
		}

		// Check if the datapoint is newer
		if dp.Timestamp().AsTime().After(existing.DataPoint.Timestamp().AsTime()) {
			state[streamDataPointID] = metrics.StreamDataPoint[DP]{
				Metric:      metric,
				DataPoint:   dp,
				LastUpdated: now,
			}
			continue
		}

		// Otherwise, we leave existing as-is
	}
}

func (p *Processor) exportMetrics() {
	p.stateLock.Lock()
	defer p.stateLock.Unlock()

	// We have to generate our own metrics slice to send to the nextConsumer
	md := pmetric.NewMetrics()

	// We want to avoid generating duplicate ResourceMetrics, ScopeMetrics, and Metrics
	// So we use lookups to only generate what we need
	rmLookup := map[identity.Resource]pmetric.ResourceMetrics{}
	smLookup := map[identity.Scope]pmetric.ScopeMetrics{}
	mLookup := map[identity.Metric]pmetric.Metric{}

	for dataID, dp := range p.numbers {
		m := getOrCreateMetric(dataID, dp.Metric, md, rmLookup, smLookup, mLookup)

		sum := m.Sum()
		numDP := sum.DataPoints().AppendEmpty()
		dp.DataPoint.CopyTo(numDP)
	}

	for dataID, dp := range p.histograms {
		m := getOrCreateMetric(dataID, dp.Metric, md, rmLookup, smLookup, mLookup)

		histogram := m.Histogram()
		histogramDP := histogram.DataPoints().AppendEmpty()
		dp.DataPoint.CopyTo(histogramDP)
	}

	for dataID, dp := range p.expHistograms {
		m := getOrCreateMetric(dataID, dp.Metric, md, rmLookup, smLookup, mLookup)

		expHistogram := m.ExponentialHistogram()
		expHistogramDP := expHistogram.DataPoints().AppendEmpty()
		dp.DataPoint.CopyTo(expHistogramDP)
	}

	if err := p.nextConsumer.ConsumeMetrics(p.ctx, md); err != nil {
		p.logger.Error("Metrics export failed", zap.Error(err))
	}

	// Clear everything now that we've exported
	p.numbers = map[identity.Stream]metrics.StreamDataPoint[pmetric.NumberDataPoint]{}
	p.histograms = map[identity.Stream]metrics.StreamDataPoint[pmetric.HistogramDataPoint]{}
	p.expHistograms = map[identity.Stream]metrics.StreamDataPoint[pmetric.ExponentialHistogramDataPoint]{}
}

func getOrCreateMetric(
	streamID identity.Stream, metricRef metrics.Metric,
	md pmetric.Metrics,
	rmLookup map[identity.Resource]pmetric.ResourceMetrics,
	smLookup map[identity.Scope]pmetric.ScopeMetrics,
	mLookup map[identity.Metric]pmetric.Metric,
) pmetric.Metric {
	// Find the ResourceMetrics
	rm, ok := rmLookup[streamID.Metric().Scope().Resource()]
	if !ok {
		// We need to create it
		rm = md.ResourceMetrics().AppendEmpty()
		metricRef.CopyToResourceMetric(rm)
	}

	// Find the ScopeMetrics
	sm, ok := smLookup[streamID.Metric().Scope()]
	if !ok {
		sm = rm.ScopeMetrics().AppendEmpty()
		metricRef.CopyToScopeMetric(sm)
	}

	m, ok := mLookup[streamID.Metric()]
	if !ok {
		m = sm.Metrics().AppendEmpty()
		metricRef.CopyToPMetric(m)
	}

	return m
}
