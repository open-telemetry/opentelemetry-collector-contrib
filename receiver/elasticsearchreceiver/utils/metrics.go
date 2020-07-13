package utils

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

func PrepareGauge(metricName string, labels map[string]string, metricValue *int64) *metricspb.Metric {
	if metricValue == nil {
		return nil
	}

	labelKeys, labelVals := buildLabels(labels, nil)

	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:      metricName,
			Type:      metricspb.MetricDescriptor_GAUGE_INT64,
			LabelKeys: labelKeys,
		},
		// One or more timeseries for a single metric, where each timeseries has
		// one or more points.
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: labelVals,
				Points: []*metricspb.Point{
					{
						Value: &metricspb.Point_Int64Value{Int64Value: *metricValue},
					},
				},
			},
		},
	}
}

func PrepareGaugeF(metricName string, labels map[string]string, metricValue *float64) *metricspb.Metric {
	if metricValue == nil {
		return nil
	}

	labelKeys, labelVals := buildLabels(labels, nil)

	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:      metricName,
			Type:      metricspb.MetricDescriptor_GAUGE_DOUBLE,
			LabelKeys: labelKeys,
		},
		// One or more timeseries for a single metric, where each timeseries has
		// one or more points.
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: labelVals,
				Points: []*metricspb.Point{
					{
						Value: &metricspb.Point_DoubleValue{DoubleValue: *metricValue},
					},
				},
			},
		},
	}
}

func PrepareCumulative(metricName string, labels map[string]string, metricValue *int64) *metricspb.Metric {
	if metricValue == nil {
		return nil
	}

	labelKeys, labelVals := buildLabels(labels, nil)

	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:      metricName,
			Type:      metricspb.MetricDescriptor_CUMULATIVE_INT64,
			LabelKeys: labelKeys,
		},
		// One or more timeseries for a single metric, where each timeseries has
		// one or more points.
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: labelVals,
				Points: []*metricspb.Point{
					{
						Value: &metricspb.Point_Int64Value{Int64Value: *metricValue},
					},
				},
			},
		},
	}
}

func buildLabels(labels map[string]string, descriptions map[string]string) (
	[]*metricspb.LabelKey, []*metricspb.LabelValue,
) {
	var keys []*metricspb.LabelKey
	var values []*metricspb.LabelValue
	for key, val := range labels {
		labelKey := &metricspb.LabelKey{Key: key}
		desc, hasDesc := descriptions[key]
		if hasDesc {
			labelKey.Description = desc
		}
		keys = append(keys, labelKey)
		values = append(values, &metricspb.LabelValue{
			Value:    val,
			HasValue: true,
		})
	}
	return keys, values
}
