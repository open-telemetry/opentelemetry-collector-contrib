package starttimeattribute

import (
	"context"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/datapointstorage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	// Type is the value users can use to configure the start time metric adjuster.
	Type = "start_time_attribute"
)

type Adjuster struct {
	set       component.TelemetrySettings
	apiConfig k8sconfig.APIConfig
	podClient podClient

	referenceCache *datapointstorage.Cache

	opts common.AdjustmentOptions
}

type podClientFactory func(context.Context, k8sconfig.APIConfig, informerFilter, bool) (podClient, error)

// NewAdjuster returns a new Adjuster which adjust metrics' start times based on the initial received points.
func NewAdjuster(set component.TelemetrySettings, attributeFilterConfig AttributesFilterConfig, useContainerReadiness bool, opts common.AdjustmentOptions) (*Adjuster, error) {
	return NewAdjusterWithFactory(set, newK8sPodClient, attributeFilterConfig, useContainerReadiness, opts)
}

// NewAdjusterWithFactory returns a new Adjuster with a custom pod client factory
func NewAdjusterWithFactory(set component.TelemetrySettings, factory podClientFactory, attributeFilterConfig AttributesFilterConfig, useContainerReadiness bool, opts common.AdjustmentOptions) (*Adjuster, error) {
	apiConfig := k8sconfig.APIConfig{
		AuthType: k8sconfig.AuthTypeServiceAccount,
	}

	ctx := context.Background()
	k8sInformerFilter := toInformerFilter(attributeFilterConfig)
	client, err := factory(ctx, apiConfig, k8sInformerFilter, useContainerReadiness)
	if err != nil {
		return nil, fmt.Errorf("failed to create pod client: %w", err)
	}

	return &Adjuster{
		apiConfig:      apiConfig,
		podClient:      client,
		set:            set,
		opts:           opts,
		referenceCache: datapointstorage.NewCache(opts.GCInterval),
	}, nil
}

func (a *Adjuster) AdjustMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	resourceMetrics := metrics.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		rm := resourceMetrics.At(i)
		resource := rm.Resource()
		// Try to extract pod identifier from resource attributes
		podID := a.extractPodIdentifier(resource.Attributes())
		if podID == nil {
			continue
		}
		attrHash := pdatautil.MapHash(rm.Resource().Attributes())
		tsm, _ := a.referenceCache.Get(attrHash)
		tsm.Lock()
		scopeMetrics := rm.ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			sm := scopeMetrics.At(j)
			metrics := sm.Metrics()

			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)

				metricName := metric.Name()
				// Only process cumulative metrics
				if !a.isCumulativeMetric(metric) {
					a.set.Logger.Debug("metric is not cumulative, skipping",
						zap.String("metricName", metricName))
					continue
				}
				if a.opts.Filter != nil && !a.opts.Filter.Matches(metricName) {
					a.set.Logger.Debug("metric not included by filter, skipping",
						zap.String("metricName", metricName))
					continue
				}
				if a.opts.SkipIfCTExists && common.HasStartTimeSet(metric) {
					continue
				}
				startTime := a.podClient.GetPodStartTime(ctx, *podID)
				if startTime.IsZero() {
					a.set.Logger.Debug("no known start time for pod",
						zap.Stringer("podIDTypee", podID.Type),
						zap.String("podID", podID.Value),
						zap.String("metricName", metricName),
					)
					continue
				}
				a.setStartTimeForMetric(tsm, metric, startTime)
			}
		}
		tsm.Unlock()
	}

	return metrics, nil
}

func (a *Adjuster) extractPodIdentifier(attrs pcommon.Map) *podIdentifier {
	podNameVal, nameOk := attrs.Get("k8s.pod.name")
	namespaceVal, nsOk := attrs.Get("k8s.namespace.name")
	if nameOk && nsOk {
		return &podIdentifier{
			Value: fmt.Sprintf("%s/%s", namespaceVal.AsString(), podNameVal.AsString()),
			Type:  podName,
		}
	}
	if uidVal, ok := attrs.Get("k8s.pod.uid"); ok {
		return &podIdentifier{
			Value: uidVal.AsString(),
			Type:  podUID,
		}
	}
	if ipVal, ok := attrs.Get("k8s.pod.ip"); ok {
		return &podIdentifier{
			Value: ipVal.AsString(),
			Type:  podIP,
		}
	}

	return nil
}

func (a *Adjuster) isCumulativeMetric(metric pmetric.Metric) bool {
	switch metric.Type() {
	case pmetric.MetricTypeSummary:
		return true
	case pmetric.MetricTypeSum:
		return metric.Sum().AggregationTemporality() == pmetric.AggregationTemporalityCumulative
	case pmetric.MetricTypeHistogram:
		return metric.Histogram().AggregationTemporality() == pmetric.AggregationTemporalityCumulative
	case pmetric.MetricTypeExponentialHistogram:
		return metric.ExponentialHistogram().AggregationTemporality() == pmetric.AggregationTemporalityCumulative
	default:
		return false
	}
}

func (a *Adjuster) setStartTimeForMetric(tsm *datapointstorage.TimeseriesMap, metric pmetric.Metric, startTime time.Time) {
	startTimeNanos := pcommon.NewTimestampFromTime(startTime)
	switch metric.Type() {
	case pmetric.MetricTypeSummary:
		dataPoints := metric.Summary().DataPoints()
		for i := 0; i < dataPoints.Len(); i++ {
			dp := dataPoints.At(i)
			refTsi, found := tsm.Get(metric, dp.Attributes())
			if !found {
				refTsi.Summary = datapointstorage.SummaryInfo{StartTime: startTimeNanos}
			} else if refTsi.IsResetSummary(dp) {
				refTsi.Summary.StartTime = pcommon.NewTimestampFromTime(dp.Timestamp().AsTime().Add(-1 * time.Millisecond))
			}
			refTsi.Summary.PreviousCount, refTsi.Summary.PreviousSum = dp.Count(), dp.Sum()
			dp.SetStartTimestamp(refTsi.Summary.StartTime)
		}
	case pmetric.MetricTypeSum:
		dataPoints := metric.Sum().DataPoints()
		for i := 0; i < dataPoints.Len(); i++ {
			dp := dataPoints.At(i)
			refTsi, found := tsm.Get(metric, dp.Attributes())
			if !found {
				refTsi.Number = datapointstorage.NumberInfo{StartTime: startTimeNanos}
			} else if refTsi.IsResetSum(dp) {
				refTsi.Number.StartTime = pcommon.NewTimestampFromTime(dp.Timestamp().AsTime().Add(-1 * time.Millisecond))
			}
			refTsi.Number.PreviousDoubleValue = dp.DoubleValue()
			refTsi.Number.PreviousIntValue = dp.IntValue()
			dp.SetStartTimestamp(startTimeNanos)
		}
	case pmetric.MetricTypeHistogram:
		dataPoints := metric.Histogram().DataPoints()
		for i := 0; i < dataPoints.Len(); i++ {
			dp := dataPoints.At(i)
			refTsi, found := tsm.Get(metric, dp.Attributes())
			if !found {
				refTsi.Histogram = datapointstorage.HistogramInfo{StartTime: startTimeNanos, ExplicitBounds: dp.ExplicitBounds().AsRaw()}
			} else if refTsi.IsResetHistogram(dp) {
				refTsi.Histogram.StartTime = pcommon.NewTimestampFromTime(dp.Timestamp().AsTime().Add(-1 * time.Millisecond))
			}
			refTsi.Histogram.PreviousCount, refTsi.Histogram.PreviousSum = dp.Count(), dp.Sum()
			refTsi.Histogram.PreviousBucketCounts = dp.BucketCounts().AsRaw()
			dp.SetStartTimestamp(refTsi.Histogram.StartTime)
		}
	case pmetric.MetricTypeExponentialHistogram:
		dataPoints := metric.ExponentialHistogram().DataPoints()
		for i := 0; i < dataPoints.Len(); i++ {
			dp := dataPoints.At(i)
			refTsi, found := tsm.Get(metric, dp.Attributes())
			if !found {
				refTsi.ExponentialHistogram = datapointstorage.ExponentialHistogramInfo{StartTime: startTimeNanos, Scale: dp.Scale()}
			} else if refTsi.IsResetExponentialHistogram(dp) {
				refTsi.ExponentialHistogram.StartTime = pcommon.NewTimestampFromTime(dp.Timestamp().AsTime().Add(-1 * time.Millisecond))
			}
			refTsi.ExponentialHistogram.PreviousPositive = datapointstorage.NewExponentialHistogramBucketInfo(dp.Positive())
			refTsi.ExponentialHistogram.PreviousNegative = datapointstorage.NewExponentialHistogramBucketInfo(dp.Negative())
			refTsi.ExponentialHistogram.PreviousCount, refTsi.ExponentialHistogram.PreviousSum, refTsi.ExponentialHistogram.PreviousZeroCount = dp.Count(), dp.Sum(), dp.ZeroCount()
			dp.SetStartTimestamp(refTsi.ExponentialHistogram.StartTime)
		}
	}
}
