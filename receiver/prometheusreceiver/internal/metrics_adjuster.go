// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// Notes on garbage collection (gc):
//
// Job-level gc:
// The Prometheus receiver will likely execute in a long running service whose lifetime may exceed
// the lifetimes of many of the jobs that it is collecting from. In order to keep the JobsMap from
// leaking memory for entries of no-longer existing jobs, the JobsMap needs to remove entries that
// haven't been accessed for a long period of time.
//
// Timeseries-level gc:
// Some jobs that the Prometheus receiver is collecting from may export timeseries based on metrics
// from other jobs (e.g. cAdvisor). In order to keep the timeseriesMap from leaking memory for entries
// of no-longer existing jobs, the timeseriesMap for each job needs to remove entries that haven't
// been accessed for a long period of time.
//
// The gc strategy uses a standard mark-and-sweep approach - each time a timeseriesMap is accessed,
// it is marked. Similarly, each time a timeseriesInfo is accessed, it is also marked.
//
// At the end of each JobsMap.get(), if the last time the JobsMap was gc'd exceeds the 'gcInterval',
// the JobsMap is locked and any timeseriesMaps that are unmarked are removed from the JobsMap
// otherwise the timeseriesMap is gc'd
//
// The gc for the timeseriesMap is straightforward - the map is locked and, for each timeseriesInfo
// in the map, if it has not been marked, it is removed otherwise it is unmarked.
//
// Alternative Strategies
// 1. If the job-level gc doesn't run often enough, or runs too often, a separate go routine can
//    be spawned at JobMap creation time that gc's at periodic intervals. This approach potentially
//    adds more contention and latency to each scrape so the current approach is used. Note that
//    the go routine will need to be cancelled upon Shutdown().
// 2. If the gc of each timeseriesMap during the gc of the JobsMap causes too much contention,
//    the gc of timeseriesMaps can be moved to the end of MetricsAdjuster().AdjustMetricSlice(). This
//    approach requires adding 'lastGC' Time and (potentially) a gcInterval duration to
//    timeseriesMap so the current approach is used instead.

// timeseriesInfo contains the information necessary to adjust from the initial point and to detect resets.
type timeseriesInfo struct {
	mark bool

	number    numberInfo
	histogram histogramInfo
	summary   summaryInfo
}

type numberInfo struct {
	startTime     pcommon.Timestamp
	previousValue float64
}

type histogramInfo struct {
	startTime     pcommon.Timestamp
	previousCount uint64
	previousSum   float64
}

type summaryInfo struct {
	startTime     pcommon.Timestamp
	previousCount uint64
	previousSum   float64
}

type timeseriesKey struct {
	name           string
	attributes     [16]byte
	aggTemporality pmetric.AggregationTemporality
}

// timeseriesMap maps from a timeseries instance (metric * label values) to the timeseries info for
// the instance.
type timeseriesMap struct {
	sync.RWMutex
	// The mutex is used to protect access to the member fields. It is acquired for the entirety of
	// AdjustMetricSlice() and also acquired by gc().

	mark   bool
	tsiMap map[timeseriesKey]*timeseriesInfo
}

// Get the timeseriesInfo for the timeseries associated with the metric and label values.
func (tsm *timeseriesMap) get(metric pmetric.Metric, kv pcommon.Map) (*timeseriesInfo, bool) {
	// This should only be invoked be functions called (directly or indirectly) by AdjustMetricSlice().
	// The lock protecting tsm.tsiMap is acquired there.
	name := metric.Name()
	key := timeseriesKey{
		name:       name,
		attributes: getAttributesSignature(kv),
	}
	switch metric.Type() {
	case pmetric.MetricTypeHistogram:
		// There are 2 types of Histograms whose aggregation temporality needs distinguishing:
		// * CumulativeHistogram
		// * GaugeHistogram
		key.aggTemporality = metric.Histogram().AggregationTemporality()
	case pmetric.MetricTypeExponentialHistogram:
		// There are 2 types of ExponentialHistograms whose aggregation temporality needs distinguishing:
		// * CumulativeHistogram
		// * GaugeHistogram
		key.aggTemporality = metric.ExponentialHistogram().AggregationTemporality()
	}

	tsm.mark = true
	tsi, ok := tsm.tsiMap[key]
	if !ok {
		tsi = &timeseriesInfo{}
		tsm.tsiMap[key] = tsi
	}
	tsi.mark = true
	return tsi, ok
}

// Create a unique string signature for attributes values sorted by attribute keys.
func getAttributesSignature(m pcommon.Map) [16]byte {
	clearedMap := pcommon.NewMap()
	for k, attrValue := range m.All() {
		value := attrValue.Str()
		if value != "" {
			clearedMap.PutStr(k, value)
		}
	}
	return pdatautil.MapHash(clearedMap)
}

// Remove timeseries that have aged out.
func (tsm *timeseriesMap) gc() {
	tsm.Lock()
	defer tsm.Unlock()
	// this shouldn't happen under the current gc() strategy
	if !tsm.mark {
		return
	}
	for ts, tsi := range tsm.tsiMap {
		if !tsi.mark {
			delete(tsm.tsiMap, ts)
		} else {
			tsi.mark = false
		}
	}
	tsm.mark = false
}

func newTimeseriesMap() *timeseriesMap {
	return &timeseriesMap{mark: true, tsiMap: map[timeseriesKey]*timeseriesInfo{}}
}

// JobsMap maps from a job instance to a map of timeseries instances for the job.
type JobsMap struct {
	sync.RWMutex
	// The mutex is used to protect access to the member fields. It is acquired for most of
	// get() and also acquired by gc().

	gcInterval time.Duration
	lastGC     time.Time
	jobsMap    map[string]*timeseriesMap
}

// NewJobsMap creates a new (empty) JobsMap.
func NewJobsMap(gcInterval time.Duration) *JobsMap {
	return &JobsMap{gcInterval: gcInterval, lastGC: time.Now(), jobsMap: make(map[string]*timeseriesMap)}
}

// Remove jobs and timeseries that have aged out.
func (jm *JobsMap) gc() {
	jm.Lock()
	defer jm.Unlock()
	// once the structure is locked, confirm that gc() is still necessary
	if time.Since(jm.lastGC) > jm.gcInterval {
		for sig, tsm := range jm.jobsMap {
			tsm.RLock()
			tsmNotMarked := !tsm.mark
			// take a read lock here, no need to get a full lock as we have a lock on the JobsMap
			tsm.RUnlock()
			if tsmNotMarked {
				delete(jm.jobsMap, sig)
			} else {
				// a full lock will be obtained in here, if required.
				tsm.gc()
			}
		}
		jm.lastGC = time.Now()
	}
}

func (jm *JobsMap) maybeGC() {
	// speculatively check if gc() is necessary, recheck once the structure is locked
	jm.RLock()
	defer jm.RUnlock()
	if time.Since(jm.lastGC) > jm.gcInterval {
		go jm.gc()
	}
}

func (jm *JobsMap) get(job, instance string) *timeseriesMap {
	sig := job + ":" + instance
	// a read lock is taken here as we will not need to modify jobsMap if the target timeseriesMap is available.
	jm.RLock()
	tsm, ok := jm.jobsMap[sig]
	jm.RUnlock()
	defer jm.maybeGC()
	if ok {
		return tsm
	}
	jm.Lock()
	defer jm.Unlock()
	// Now that we've got an exclusive lock, check once more to ensure an entry wasn't created in the interim
	// and then create a new timeseriesMap if required.
	tsm2, ok2 := jm.jobsMap[sig]
	if ok2 {
		return tsm2
	}
	tsm2 = newTimeseriesMap()
	jm.jobsMap[sig] = tsm2
	return tsm2
}

type MetricsAdjuster interface {
	AdjustMetrics(metrics pmetric.Metrics) error
}

// initialPointAdjuster takes a map from a metric instance to the initial point in the metrics instance
// and provides AdjustMetricSlice, which takes a sequence of metrics and adjust their start times based on
// the initial points.
type initialPointAdjuster struct {
	jobsMap          *JobsMap
	logger           *zap.Logger
	useCreatedMetric bool
	// usePointTimeForReset forces the adjuster to use the timestamp of the
	// point instead of the start timestamp when it detects resets.  This is
	// useful when this adjuster is used after another adjuster that
	// pre-populated start times.
	usePointTimeForReset bool
}

// NewInitialPointAdjuster returns a new MetricsAdjuster that adjust metrics' start times based on the initial received points.
func NewInitialPointAdjuster(logger *zap.Logger, gcInterval time.Duration, useCreatedMetric bool) MetricsAdjuster {
	return &initialPointAdjuster{
		jobsMap:          NewJobsMap(gcInterval),
		logger:           logger,
		useCreatedMetric: useCreatedMetric,
	}
}

// AdjustMetrics takes a sequence of metrics and adjust their start times based on the initial and
// previous points in the timeseriesMap.
func (a *initialPointAdjuster) AdjustMetrics(metrics pmetric.Metrics) error {
	if removeStartTimeAdjustment.IsEnabled() {
		return nil
	}
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		_, found := rm.Resource().Attributes().Get(string(semconv.ServiceNameKey))
		if !found {
			return errors.New("adjusting metrics without job")
		}

		_, found = rm.Resource().Attributes().Get(string(semconv.ServiceInstanceIDKey))
		if !found {
			return errors.New("adjusting metrics without instance")
		}
	}

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		job, _ := rm.Resource().Attributes().Get(string(semconv.ServiceNameKey))
		instance, _ := rm.Resource().Attributes().Get(string(semconv.ServiceInstanceIDKey))
		tsm := a.jobsMap.get(job.Str(), instance.Str())

		// The lock on the relevant timeseriesMap is held throughout the adjustment process to ensure that
		// nothing else can modify the data used for adjustment.
		tsm.Lock()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				metric := ilm.Metrics().At(k)
				switch dataType := metric.Type(); dataType {
				case pmetric.MetricTypeGauge:
					// gauges don't need to be adjusted so no additional processing is necessary

				case pmetric.MetricTypeHistogram:
					a.adjustMetricHistogram(tsm, metric)

				case pmetric.MetricTypeSummary:
					a.adjustMetricSummary(tsm, metric)

				case pmetric.MetricTypeSum:
					a.adjustMetricSum(tsm, metric)

				case pmetric.MetricTypeExponentialHistogram:
					a.adjustMetricExponentialHistogram(tsm, metric)

				case pmetric.MetricTypeEmpty:
					fallthrough

				default:
					// this shouldn't happen
					a.logger.Info("Adjust - skipping unexpected point", zap.String("type", dataType.String()))
				}
			}
		}
		tsm.Unlock()
	}
	return nil
}

func (a *initialPointAdjuster) adjustMetricHistogram(tsm *timeseriesMap, current pmetric.Metric) {
	histogram := current.Histogram()
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		return
	}

	currentPoints := histogram.DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)

		// start timestamp was set from _created
		if a.useCreatedMetric &&
			!currentDist.Flags().NoRecordedValue() &&
			currentDist.StartTimestamp() < currentDist.Timestamp() {
			continue
		}

		tsi, found := tsm.get(current, currentDist.Attributes())
		if !found {
			// initialize everything.
			tsi.histogram.startTime = currentDist.StartTimestamp()
			tsi.histogram.previousCount = currentDist.Count()
			tsi.histogram.previousSum = currentDist.Sum()
			continue
		}

		if currentDist.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentDist.SetStartTimestamp(tsi.histogram.startTime)
			continue
		}

		if currentDist.Count() < tsi.histogram.previousCount || currentDist.Sum() < tsi.histogram.previousSum {
			// reset re-initialize everything.
			tsi.histogram.startTime = currentDist.StartTimestamp()
			if a.usePointTimeForReset {
				tsi.histogram.startTime = currentDist.Timestamp()
				currentDist.SetStartTimestamp(tsi.histogram.startTime)
			}
			tsi.histogram.previousCount = currentDist.Count()
			tsi.histogram.previousSum = currentDist.Sum()
			continue
		}

		// Update only previous values.
		tsi.histogram.previousCount = currentDist.Count()
		tsi.histogram.previousSum = currentDist.Sum()
		currentDist.SetStartTimestamp(tsi.histogram.startTime)
	}
}

func (a *initialPointAdjuster) adjustMetricExponentialHistogram(tsm *timeseriesMap, current pmetric.Metric) {
	histogram := current.ExponentialHistogram()
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		// Only dealing with CumulativeDistributions.
		return
	}

	currentPoints := histogram.DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentDist := currentPoints.At(i)

		// start timestamp was set from _created
		if a.useCreatedMetric &&
			!currentDist.Flags().NoRecordedValue() &&
			currentDist.StartTimestamp() < currentDist.Timestamp() {
			continue
		}

		tsi, found := tsm.get(current, currentDist.Attributes())
		if !found {
			// initialize everything.
			tsi.histogram.startTime = currentDist.StartTimestamp()
			tsi.histogram.previousCount = currentDist.Count()
			tsi.histogram.previousSum = currentDist.Sum()
			continue
		}

		if currentDist.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentDist.SetStartTimestamp(tsi.histogram.startTime)
			continue
		}

		if currentDist.Count() < tsi.histogram.previousCount || currentDist.Sum() < tsi.histogram.previousSum {
			// reset re-initialize everything.
			tsi.histogram.startTime = currentDist.StartTimestamp()
			if a.usePointTimeForReset {
				tsi.histogram.startTime = currentDist.Timestamp()
				currentDist.SetStartTimestamp(tsi.histogram.startTime)
			}
			tsi.histogram.previousCount = currentDist.Count()
			tsi.histogram.previousSum = currentDist.Sum()
			continue
		}

		// Update only previous values.
		tsi.histogram.previousCount = currentDist.Count()
		tsi.histogram.previousSum = currentDist.Sum()
		currentDist.SetStartTimestamp(tsi.histogram.startTime)
	}
}

func (a *initialPointAdjuster) adjustMetricSum(tsm *timeseriesMap, current pmetric.Metric) {
	currentPoints := current.Sum().DataPoints()
	for i := 0; i < currentPoints.Len(); i++ {
		currentSum := currentPoints.At(i)

		// start timestamp was set from _created
		if a.useCreatedMetric &&
			!currentSum.Flags().NoRecordedValue() &&
			currentSum.StartTimestamp() < currentSum.Timestamp() {
			continue
		}

		tsi, found := tsm.get(current, currentSum.Attributes())
		if !found {
			// initialize everything.
			tsi.number.startTime = currentSum.StartTimestamp()
			tsi.number.previousValue = currentSum.DoubleValue()
			continue
		}

		if currentSum.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentSum.SetStartTimestamp(tsi.number.startTime)
			continue
		}

		if currentSum.DoubleValue() < tsi.number.previousValue {
			// reset re-initialize everything.
			tsi.number.startTime = currentSum.StartTimestamp()
			if a.usePointTimeForReset {
				tsi.number.startTime = currentSum.Timestamp()
				currentSum.SetStartTimestamp(tsi.number.startTime)
			}
			tsi.number.previousValue = currentSum.DoubleValue()
			continue
		}

		// Update only previous values.
		tsi.number.previousValue = currentSum.DoubleValue()
		currentSum.SetStartTimestamp(tsi.number.startTime)
	}
}

func (a *initialPointAdjuster) adjustMetricSummary(tsm *timeseriesMap, current pmetric.Metric) {
	currentPoints := current.Summary().DataPoints()

	for i := 0; i < currentPoints.Len(); i++ {
		currentSummary := currentPoints.At(i)

		// start timestamp was set from _created
		if a.useCreatedMetric &&
			!currentSummary.Flags().NoRecordedValue() &&
			currentSummary.StartTimestamp() < currentSummary.Timestamp() {
			continue
		}

		tsi, found := tsm.get(current, currentSummary.Attributes())
		if !found {
			// initialize everything.
			tsi.summary.startTime = currentSummary.StartTimestamp()
			tsi.summary.previousCount = currentSummary.Count()
			tsi.summary.previousSum = currentSummary.Sum()
			continue
		}

		if currentSummary.Flags().NoRecordedValue() {
			// TODO: Investigate why this does not reset.
			currentSummary.SetStartTimestamp(tsi.summary.startTime)
			continue
		}

		if (currentSummary.Count() != 0 &&
			tsi.summary.previousCount != 0 &&
			currentSummary.Count() < tsi.summary.previousCount) ||
			(currentSummary.Sum() != 0 &&
				tsi.summary.previousSum != 0 &&
				currentSummary.Sum() < tsi.summary.previousSum) {
			// reset re-initialize everything.
			tsi.summary.startTime = currentSummary.StartTimestamp()
			if a.usePointTimeForReset {
				tsi.summary.startTime = currentSummary.Timestamp()
				currentSummary.SetStartTimestamp(tsi.summary.startTime)
			}
			tsi.summary.previousCount = currentSummary.Count()
			tsi.summary.previousSum = currentSummary.Sum()
			continue
		}

		// Update only previous values.
		tsi.summary.previousCount = currentSummary.Count()
		tsi.summary.previousSum = currentSummary.Sum()
		currentSummary.SetStartTimestamp(tsi.summary.startTime)
	}
}
