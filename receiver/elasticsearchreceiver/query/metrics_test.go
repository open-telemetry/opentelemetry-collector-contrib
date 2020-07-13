package query

import (
	"testing"

	"github.com/signalfx/golib/datapoint"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// Tests datapoints collected from a bucket aggregation without
// sub metric aggregations
func TestDatapointsFromTerminalBucketAggregation(t *testing.T) {
	metrics := CollectMetrics(HTTPResponse{
		Aggregations: map[string]*aggregationResponse{
			"host": {
				Buckets: map[interface{}]*bucketResponse{
					"helsinki": {
						Key:             "helsinki",
						DocCount:        newInt64(8800),
						SubAggregations: map[string]*aggregationResponse{},
					},
					"nairobi": {
						Key:             "nairobi",
						DocCount:        newInt64(8800),
						SubAggregations: map[string]*aggregationResponse{},
					},
				},
				SubAggregations: map[string]*aggregationResponse{},
				OtherValues:     map[string]interface{}{},
			},
		},
	}, map[string]*AggregationMeta{
		"host": {
			Type: "filters",
		},
	}, map[string]string{}, zap.NewNop())

	assert.ElementsMatch(t, metrics, []*datapoint.Datapoint{
		{
			Metric: "host.doc_count",
			Dimensions: map[string]string{
				"bucket_aggregation_type": "filters",
				"host":                    "helsinki",
			},
			Value:      datapoint.NewIntValue(8800),
			MetricType: datapoint.Gauge,
		},
		{
			Metric: "host.doc_count",
			Dimensions: map[string]string{
				"bucket_aggregation_type": "filters",
				"host":                    "nairobi",
			},
			Value:      datapoint.NewIntValue(8800),
			MetricType: datapoint.Gauge,
		},
	})
}

// Tests avg aggregation under a terms aggregation
func TestMetricAggregationWithTermsAggregation(t *testing.T) {
	metrics := CollectMetrics(HTTPResponse{
		Aggregations: map[string]*aggregationResponse{
			"host": {
				Buckets: map[interface{}]*bucketResponse{
					"nairobi": {
						Key:      "nairobi",
						DocCount: newInt64(122),
						SubAggregations: map[string]*aggregationResponse{
							"metric_agg_1": {
								Value:           *newFloat64(48.41803278688525),
								SubAggregations: map[string]*aggregationResponse{},
								OtherValues:     map[string]interface{}{},
							},
						},
					},
					"helsinki": {
						Key:      "helsinki",
						DocCount: newInt64(126),
						SubAggregations: map[string]*aggregationResponse{
							"metric_agg_1": {
								Value:           *newFloat64(49.357142857142854),
								SubAggregations: map[string]*aggregationResponse{},
								OtherValues:     map[string]interface{}{},
							},
						},
					},
				},
				SubAggregations: map[string]*aggregationResponse{},
				OtherValues:     map[string]interface{}{},
			},
		},
	}, map[string]*AggregationMeta{
		"host": {
			Type: "terms",
		},
		"metric_agg_1": {
			Type: "avg",
		},
	}, map[string]string{}, zap.NewNop())

	assert.ElementsMatch(t, metrics, []*datapoint.Datapoint{
		{
			Metric: "metric_agg_1",
			Dimensions: map[string]string{
				"metric_aggregation_type": "avg",
				"host":                    "nairobi",
			},
			Value:      datapoint.NewFloatValue(48.41803278688525),
			MetricType: datapoint.Gauge,
		},
		{
			Metric: "metric_agg_1",
			Dimensions: map[string]string{
				"metric_aggregation_type": "avg",
				"host":                    "helsinki",
			},
			Value:      datapoint.NewFloatValue(49.357142857142854),
			MetricType: datapoint.Gauge,
		},
	})
}

// Tests datapoints from extended_stats aggregation within bucket aggregation
func TestExtendedStatsAggregationsFromFiltersAggregation(t *testing.T) {
	metrics := CollectMetrics(HTTPResponse{
		Aggregations: map[string]*aggregationResponse{
			"host": {
				Buckets: map[interface{}]*bucketResponse{
					"nairobi": {
						Key:      "nairobi",
						DocCount: newInt64(5134),
						SubAggregations: map[string]*aggregationResponse{
							"metric_agg_1": {
								SubAggregations: map[string]*aggregationResponse{},
								OtherValues: map[string]interface{}{
									"count":          5134.0,
									"min":            0.0,
									"max":            100.0,
									"avg":            50.14530580444098,
									"sum":            257446.0,
									"sum_of_squares": 1.7184548e7,
									"variance":       832.6528246727477,
									"std_deviation":  28.855724296450223,
									"std_deviation_bounds": map[string]interface{}{
										"upper": 107.85675439734143,
										"lower": -7.566142788459466,
									},
								},
							},
						},
					},
					"madrid": {
						Key:      "madrid",
						DocCount: newInt64(5134),
						SubAggregations: map[string]*aggregationResponse{
							"metric_agg_1": {
								SubAggregations: map[string]*aggregationResponse{},
								OtherValues: map[string]interface{}{
									"count":          5134.0,
									"min":            0.0,
									"max":            100.0,
									"avg":            50.03486560186989,
									"sum":            256879.0,
									"sum_of_squares": 1.7288541e7,
									"variance":       863.9724891034797,
									"std_deviation":  29.39340893981982,
									"std_deviation_bounds": map[string]interface{}{
										"upper": 108.82168348150952,
										"lower": -8.751952277769753,
									},
								},
							},
						},
					},
				},
				SubAggregations: map[string]*aggregationResponse{},
				OtherValues:     map[string]interface{}{},
			},
		},
	}, map[string]*AggregationMeta{
		"host": {
			Type: "filters",
		},
		"metric_agg_1": {
			Type: "extended_stats",
		},
	}, map[string]string{}, zap.NewNop())

	dims := map[string]map[string]string{
		"madrid": {
			"host":                    "madrid",
			"metric_aggregation_type": "extended_stats",
		},
		"nairobi": {
			"host":                    "nairobi",
			"metric_aggregation_type": "extended_stats",
		},
	}

	assert.ElementsMatch(t, metrics, []*datapoint.Datapoint{
		{
			Metric:     "metric_agg_1.count",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(5134.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.min",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(0.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.max",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(100.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.avg",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(50.03486560186989),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.sum",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(256879.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.sum_of_squares",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(1.7288541e7),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.variance",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(863.9724891034797),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.std_deviation",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(29.39340893981982),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.std_deviation_bounds.lower",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(-8.751952277769753),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.std_deviation_bounds.upper",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(108.82168348150952),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.count",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(5134.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.min",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(0.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.max",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(100.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.avg",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(50.14530580444098),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.sum",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(257446.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.sum_of_squares",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(1.7184548e7),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.variance",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(832.6528246727477),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.std_deviation",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(28.855724296450223),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.std_deviation_bounds.lower",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(-7.566142788459466),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.std_deviation_bounds.upper",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(107.85675439734143),
			MetricType: datapoint.Gauge,
		},
	})
}

// Tests datapoints from percentiles aggregation within bucket aggregation
func TestPercentilesAggregationsFromFiltersAggregation(t *testing.T) {
	metrics := CollectMetrics(HTTPResponse{
		Aggregations: map[string]*aggregationResponse{
			"host": {
				Buckets: map[interface{}]*bucketResponse{
					"nairobi": {
						Key:      "nairobi",
						DocCount: newInt64(5134),
						SubAggregations: map[string]*aggregationResponse{
							"metric_agg_1": {
								Values: map[string]interface{}{
									"50.0": 50.0,
									"75.0": 75.0,
									"99.0": 99.07999999999993,
								},
								SubAggregations: map[string]*aggregationResponse{},
								OtherValues:     map[string]interface{}{},
							},
						},
					},
					"madrid": {
						Key:      "madrid",
						DocCount: newInt64(5134),
						SubAggregations: map[string]*aggregationResponse{
							"metric_agg_1": {
								Values: map[string]interface{}{
									"50.0": 50.294871794871796,
									"75.0": 75.98039215686275,
									"99.0": 100.0,
								},
								SubAggregations: map[string]*aggregationResponse{},
								OtherValues:     map[string]interface{}{},
							},
						},
					},
				},
				SubAggregations: map[string]*aggregationResponse{},
				OtherValues:     map[string]interface{}{},
			},
		},
	}, map[string]*AggregationMeta{
		"host": {
			Type: "filters",
		},
		"metric_agg_1": {
			Type: "percentiles",
		},
	}, map[string]string{}, zap.NewNop())

	dims := map[string]map[string]string{
		"madrid": {
			"host":                    "madrid",
			"metric_aggregation_type": "percentiles",
		},
		"nairobi": {
			"host":                    "nairobi",
			"metric_aggregation_type": "percentiles",
		},
	}

	assert.ElementsMatch(t, metrics, []*datapoint.Datapoint{
		{
			Metric:     "metric_agg_1.p50",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(50.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.p75",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(75.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.p99",
			Dimensions: dims["nairobi"],
			Value:      datapoint.NewFloatValue(99.07999999999993),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.p50",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(50.294871794871796),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.p75",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(75.98039215686275),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.p99",
			Dimensions: dims["madrid"],
			Value:      datapoint.NewFloatValue(100.0),
			MetricType: datapoint.Gauge,
		},
	})
}

// Tests multiple metric aggregation in a bucket aggregation
func TestMultipleMetricAggregationWithTermsAggregation(t *testing.T) {
	metrics := CollectMetrics(HTTPResponse{
		Aggregations: map[string]*aggregationResponse{
			"host": {
				Buckets: map[interface{}]*bucketResponse{
					"nairobi": {
						Key:      "nairobi",
						DocCount: newInt64(5134),
						SubAggregations: map[string]*aggregationResponse{
							"metric_agg_1": {
								Values: map[string]interface{}{
									"50.0": 50.0,
									"75.0": 75.0,
									"99.0": 99.07999999999993,
								},
								SubAggregations: map[string]*aggregationResponse{},
								OtherValues:     map[string]interface{}{},
							},
							"metric_agg_2": {
								Value:           *newFloat64(48.41803278688525),
								SubAggregations: map[string]*aggregationResponse{},
								OtherValues:     map[string]interface{}{},
							},
							"metric_agg_3": {
								SubAggregations: map[string]*aggregationResponse{},
								OtherValues: map[string]interface{}{
									"count":          5134.0,
									"min":            0.0,
									"max":            100.0,
									"avg":            50.14530580444098,
									"sum":            257446.0,
									"sum_of_squares": 1.7184548e7,
									"variance":       832.6528246727477,
									"std_deviation":  28.855724296450223,
									"std_deviation_bounds": map[string]interface{}{
										"upper": 107.85675439734143,
										"lower": -7.566142788459466,
									},
								},
							},
						},
					},
				},
				SubAggregations: map[string]*aggregationResponse{},
				OtherValues:     map[string]interface{}{},
			},
		},
	}, map[string]*AggregationMeta{
		"host": {
			Type: "filters",
		},
		"metric_agg_1": {
			Type: "percentiles",
		},
		"metric_agg_2": {
			Type: "avg",
		},
		"metric_agg_3": {
			Type: "extended_stats",
		},
	}, map[string]string{}, zap.NewNop())

	dims := map[string]map[string]string{
		"metric_agg_1": {
			"host":                    "nairobi",
			"metric_aggregation_type": "percentiles",
		},
		"metric_agg_2": {
			"host":                    "nairobi",
			"metric_aggregation_type": "avg",
		},
		"metric_agg_3": {
			"host":                    "nairobi",
			"metric_aggregation_type": "extended_stats",
		},
	}

	assert.ElementsMatch(t, metrics, []*datapoint.Datapoint{
		{
			Metric:     "metric_agg_1.p50",
			Dimensions: dims["metric_agg_1"],
			Value:      datapoint.NewFloatValue(50.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.p75",
			Dimensions: dims["metric_agg_1"],
			Value:      datapoint.NewFloatValue(75.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_1.p99",
			Dimensions: dims["metric_agg_1"],
			Value:      datapoint.NewFloatValue(99.07999999999993),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_2",
			Dimensions: dims["metric_agg_2"],
			Value:      datapoint.NewFloatValue(48.41803278688525),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_3.count",
			Dimensions: dims["metric_agg_3"],
			Value:      datapoint.NewFloatValue(5134.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_3.min",
			Dimensions: dims["metric_agg_3"],
			Value:      datapoint.NewFloatValue(0.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_3.max",
			Dimensions: dims["metric_agg_3"],
			Value:      datapoint.NewFloatValue(100.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_3.avg",
			Dimensions: dims["metric_agg_3"],
			Value:      datapoint.NewFloatValue(50.14530580444098),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_3.sum",
			Dimensions: dims["metric_agg_3"],
			Value:      datapoint.NewFloatValue(257446.0),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_3.sum_of_squares",
			Dimensions: dims["metric_agg_3"],
			Value:      datapoint.NewFloatValue(1.7184548e7),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_3.variance",
			Dimensions: dims["metric_agg_3"],
			Value:      datapoint.NewFloatValue(832.6528246727477),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_3.std_deviation",
			Dimensions: dims["metric_agg_3"],
			Value:      datapoint.NewFloatValue(28.855724296450223),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_3.std_deviation_bounds.lower",
			Dimensions: dims["metric_agg_3"],
			Value:      datapoint.NewFloatValue(-7.566142788459466),
			MetricType: datapoint.Gauge,
		},
		{
			Metric:     "metric_agg_3.std_deviation_bounds.upper",
			Dimensions: dims["metric_agg_3"],
			Value:      datapoint.NewFloatValue(107.85675439734143),
			MetricType: datapoint.Gauge,
		},
	})
}
