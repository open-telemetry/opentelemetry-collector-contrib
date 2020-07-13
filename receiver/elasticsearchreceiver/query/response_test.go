package query

import (
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/go-errors/errors"
)

/*
All tests are performed against outputs from an index where documents have the following
structure:

{
   'cpu_utilization':87,
   'memory_utilization':94,
   'host':'helsinki',
   'service':'android',
   'container_id':'macbook',
   '@timestamp':1580321240579
}
*/

// Tests for single value metric aggregations inside a multi value
// bucket aggregation
func TestSimpleSingleValueAggregation(t *testing.T) {
	contexts, err := getSimpleSingleValueAggregationTestContexts()

	if err != nil {
		t.Fatalf("Failed to load test contexts: %v", err)
	}

	for _, context := range contexts {
		if !reflect.DeepEqual(context.actual.Aggregations, context.expected.Aggregations) {
			t.Errorf("Expected and actual responses do not match for %s", context.rawFile)
		}
	}
}

// Tests for multi value metric aggregations inside a multi value
// bucket aggregation
func TestSimpleMultiValueAggregation(t *testing.T) {
	contexts, err := getSimpleMultiValueAggregationTestContexts()

	if err != nil {
		t.Fatalf("Failed to load test contexts: %v", err)
	}

	for _, context := range contexts {
		if !reflect.DeepEqual(context.actual.Aggregations, context.expected.Aggregations) {
			t.Errorf("Expected and actual responses do not match for %s", context.rawFile)
		}
	}
}

// Tests for multiple metric aggregations inside a multi value
// bucket aggregation
func TestMulitpleMetricAggregations(t *testing.T) {
	contexts, err := getMultipleMetricAggregationTestContexts()

	if err != nil {
		t.Fatalf("Failed to load test contexts: %v", err)
	}

	for _, context := range contexts {
		if !reflect.DeepEqual(context.actual.Aggregations, context.expected.Aggregations) {
			t.Errorf("Expected and actual responses do not match for %s", context.rawFile)
		}
	}
}

// Tests for bucket aggregations without a sub metric aggregation
func TestBucketAggregationWithoutSubMetricAggregation(t *testing.T) {
	contexts, err := getTerminalBucketAggregationTestContexts()

	if err != nil {
		t.Fatalf("Failed to load test contexts: %v", err)
	}

	for _, context := range contexts {
		if !reflect.DeepEqual(context.actual.Aggregations, context.expected.Aggregations) {
			t.Errorf("Expected and actual responses do not match for %s", context.rawFile)
		}
	}
}

// Tests for date histogram aggregations
func TestDateHistogramAggregation(t *testing.T) {
	contexts, err := getDateHistogramAggregationTestContexts()

	if err != nil {
		t.Fatalf("Failed to load test contexts: %v", err)
	}

	for _, context := range contexts {
		if !reflect.DeepEqual(context.actual.Aggregations, context.expected.Aggregations) {
			t.Errorf("Expected and actual responses do not match for %s", context.rawFile)
		}
	}
}

// Tests for single bucket filter aggregation
func TestSingleBucketFilterAggregation(t *testing.T) {
	contexts, err := getSingleBucketFilterAggregationTestContexts()

	if err != nil {
		t.Fatalf("Failed to load test contexts: %v", err)
	}

	for _, context := range contexts {
		if !reflect.DeepEqual(context.actual.Aggregations, context.expected.Aggregations) {
			t.Errorf("Expected and actual responses do not match for %s", context.rawFile)
		}
	}
}

// Tests for metric aggreations within histogram aggregations
func TestHistogramAggregation(t *testing.T) {
	contexts, err := getHistogramAggregationTestContexts()

	if err != nil {
		t.Fatalf("Failed to load test contexts: %v", err)
	}

	for _, context := range contexts {
		if !reflect.DeepEqual(context.actual.Aggregations, context.expected.Aggregations) {
			t.Errorf("Expected and actual responses do not match for %s", context.rawFile)
		}
	}
}

// Tests for metric aggreations within adjacency matrix aggregations
func TestAdjacencyAggregation(t *testing.T) {
	contexts, err := getAdjacenyMatrixAggregationTestContexts()

	if err != nil {
		t.Fatalf("Failed to load test contexts: %v", err)
	}

	for _, context := range contexts {
		if !reflect.DeepEqual(context.actual.Aggregations, context.expected.Aggregations) {
			t.Errorf("Expected and actual responses do not match for %s", context.rawFile)
		}
	}
}

type testContext struct {
	rawFile  string
	actual   HTTPResponse
	expected HTTPResponse
}

func getAdjacenyMatrixAggregationTestContexts() ([]*testContext, *errors.Error) {
	filePaths := []string{
		"testdata/simple_avg_aggs_with_adjacency_matrix_aggs.json",
	}

	expected := []HTTPResponse{
		{
			Aggregations: aggregationsMap{
				"host_groups": {
					Buckets: bucketsMap{
						"group1": {
							Key:      "group1",
							DocCount: newInt64(60),
							SubAggregations: aggregationsMap{
								"metric_agg": {
									Value:           51.06666666666667,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						"group2": {
							Key:      "group2",
							DocCount: newInt64(120),
							SubAggregations: aggregationsMap{
								"metric_agg": {
									Value:           47.86666666666667,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						"group3": {
							Key:      "group3",
							DocCount: newInt64(60),
							SubAggregations: aggregationsMap{
								"metric_agg": {
									Value:           54.333333333333336,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
					},
					SubAggregations: aggregationsMap{},
					OtherValues:     map[string]interface{}{},
				},
			},
		},
	}

	return getAggregationTestContexts(filePaths, expected)
}

func getHistogramAggregationTestContexts() ([]*testContext, *errors.Error) {
	filePaths := []string{
		"testdata/simple_avg_aggs_with_histogram_aggs.json",
	}

	expected := []HTTPResponse{
		{
			Aggregations: aggregationsMap{
				"cpu_values": {
					Buckets: bucketsMap{
						0.0: {
							Key:      0.0,
							DocCount: newInt64(59),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           10.559322033898304,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						20.0: {
							Key:      20.0,
							DocCount: newInt64(35),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           30.571428571428573,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						40.0: {
							Key:      40.0,
							DocCount: newInt64(57),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           48.80701754385965,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						60.0: {
							Key:      60.0,
							DocCount: newInt64(42),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           69.64285714285714,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						80.0: {
							Key:      80.0,
							DocCount: newInt64(45),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           89.4,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						100.0: {
							Key:      100.0,
							DocCount: newInt64(2),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           100.0,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
					},
					SubAggregations: aggregationsMap{},
					OtherValues:     map[string]interface{}{},
				},
			},
		},
	}

	return getAggregationTestContexts(filePaths, expected)
}

func getSingleBucketFilterAggregationTestContexts() ([]*testContext, *errors.Error) {
	filePaths := []string{
		"testdata/simple_avg_aggs_with_filter_aggs.json",
	}

	expected := []HTTPResponse{
		{
			Aggregations: aggregationsMap{
				"favourite_host": {
					DocCount: newInt64(9398),
					SubAggregations: aggregationsMap{
						"metric_agg_1": {
							Value:           49.55799106192807,
							SubAggregations: aggregationsMap{},
							OtherValues:     map[string]interface{}{},
						},
					},
					OtherValues: map[string]interface{}{},
				},
			},
		},
	}

	return getAggregationTestContexts(filePaths, expected)
}

func getDateHistogramAggregationTestContexts() ([]*testContext, *errors.Error) {
	filePaths := []string{
		"testdata/simple_avg_aggs_with_date_histogram.json",
	}

	expected := []HTTPResponse{
		{
			Aggregations: aggregationsMap{
				"bydate": {
					Buckets: bucketsMap{
						float64(1580318160000): {
							Key:      float64(1580318160000),
							DocCount: newInt64(32),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           50.4375,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						float64(1580318220000): {
							Key:      float64(1580318220000),
							DocCount: newInt64(48),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           51.125,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						float64(1580318280000): {
							Key:      float64(1580318280000),
							DocCount: newInt64(48),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           51.166666666666664,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						float64(1580318340000): {
							Key:      float64(1580318340000),
							DocCount: newInt64(48),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           45.354166666666664,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						float64(1580318400000): {
							Key:      float64(1580318400000),
							DocCount: newInt64(48),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           55.916666666666664,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						float64(1580318460000): {
							Key:      float64(1580318460000),
							DocCount: newInt64(16),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           50.625,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
					},
					SubAggregations: aggregationsMap{},
					OtherValues:     map[string]interface{}{},
				},
			},
		},
	}

	return getAggregationTestContexts(filePaths, expected)
}

func getTerminalBucketAggregationTestContexts() ([]*testContext, *errors.Error) {
	filePaths := []string{
		"testdata/terminal_bucket_aggs_with_filters_aggs.json",
	}

	expected := []HTTPResponse{
		{
			Aggregations: aggregationsMap{
				"host": {
					Buckets: bucketsMap{
						"helsinki": {
							Key:             "helsinki",
							DocCount:        newInt64(8800),
							SubAggregations: aggregationsMap{},
						},
						"nairobi": {
							Key:             "nairobi",
							DocCount:        newInt64(8800),
							SubAggregations: aggregationsMap{},
						},
					},
					SubAggregations: aggregationsMap{},
					OtherValues:     map[string]interface{}{},
				},
			},
		},
	}

	return getAggregationTestContexts(filePaths, expected)
}

func getSimpleSingleValueAggregationTestContexts() ([]*testContext, *errors.Error) {
	filePaths := []string{
		"testdata/simple_avg_aggs_with_filters_aggs.json",
		"testdata/simple_avg_aggs_with_terms_aggs.json",
	}

	expected := []HTTPResponse{
		{
			Aggregations: aggregationsMap{
				"host": {
					Buckets: bucketsMap{
						"helsinki": {
							Key:      "helsinki",
							DocCount: newInt64(126),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           *newFloat64(49.357142857142854),
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						"nairobi": {
							Key:      "nairobi",
							DocCount: newInt64(126),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           *newFloat64(49.04761904761905),
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
					},
					SubAggregations: aggregationsMap{},
					OtherValues:     map[string]interface{}{},
				},
			},
		},
		{
			Aggregations: aggregationsMap{
				"host": {
					Buckets: bucketsMap{
						"helsinki": {
							Key:      "helsinki",
							DocCount: newInt64(122),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           *newFloat64(49.622950819672134),
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						"lisbon": {
							Key:      "lisbon",
							DocCount: newInt64(122),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           *newFloat64(43.23770491803279),
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						"madrid": {
							Key:      "madrid",
							DocCount: newInt64(122),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           *newFloat64(50.50819672131148),
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						"nairobi": {
							Key:      "nairobi",
							DocCount: newInt64(122),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Value:           *newFloat64(48.41803278688525),
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
					},
					SubAggregations: aggregationsMap{},
					OtherValues:     map[string]interface{}{},
				},
			},
		},
	}
	return getAggregationTestContexts(filePaths, expected)
}

func getSimpleMultiValueAggregationTestContexts() ([]*testContext, *errors.Error) {
	filePaths := []string{
		"testdata/simple_stats_aggs_with_filters_aggs.json",
		"testdata/simple_extended_stats_aggs_with_filters_aggs.json",
		"testdata/simple_percentiles_aggs_with_filters_aggs.json",
		"testdata/simple_percentiles_aggs_with_terms_aggs.json",
	}

	expected := []HTTPResponse{
		{
			Aggregations: aggregationsMap{
				"host": {
					Buckets: bucketsMap{
						"helsinki": {
							Key:      "helsinki",
							DocCount: newInt64(4344),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									SubAggregations: aggregationsMap{},
									OtherValues: map[string]interface{}{
										"count": 4344.0,
										"min":   0.0,
										"max":   100.00,
										"avg":   49.2847605893186,
										"sum":   214093.0,
									},
								},
							},
						},
						"nairobi": {
							Key:      "nairobi",
							DocCount: newInt64(4344),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									SubAggregations: aggregationsMap{},
									OtherValues: map[string]interface{}{
										"count": 4344.0,
										"min":   0.0,
										"max":   100.0,
										"avg":   50.208333333333336,
										"sum":   218105.0,
									},
								},
							},
						},
					},
					SubAggregations: aggregationsMap{},
					OtherValues:     map[string]interface{}{},
				},
			},
		},
		{
			Aggregations: aggregationsMap{
				"host": {
					Buckets: bucketsMap{
						"helsinki": {
							Key:      "helsinki",
							DocCount: newInt64(4362),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									SubAggregations: aggregationsMap{},
									OtherValues: map[string]interface{}{
										"count":          4362.0,
										"min":            0.0,
										"max":            100.0,
										"avg":            49.27212287941311,
										"sum":            214925.0,
										"sum_of_squares": 1.4335299E7,
										"variance":       858.662996364543,
										"std_deviation":  29.302952007682485,
										"std_deviation_bounds": map[string]interface{}{
											"upper": 107.87802689477809,
											"lower": -9.33378113595186,
										},
									},
								},
							},
						},
						"nairobi": {
							Key:      "nairobi",
							DocCount: newInt64(4362),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									SubAggregations: aggregationsMap{},
									OtherValues: map[string]interface{}{
										"count":          4362.0,
										"min":            0.0,
										"max":            100.0,
										"avg":            50.197615772581386,
										"sum":            218962.0,
										"sum_of_squares": 1.4644106E7,
										"variance":       837.3992790472341,
										"std_deviation":  28.93785201163407,
										"std_deviation_bounds": map[string]interface{}{
											"upper": 108.07331979584953,
											"lower": -7.678088250686756,
										},
									},
								},
							},
						},
					},
					SubAggregations: aggregationsMap{},
					OtherValues:     map[string]interface{}{},
				},
			},
		},
		{
			Aggregations: aggregationsMap{
				"host": {
					Buckets: bucketsMap{
						"helsinki": {
							Key:      "helsinki",
							DocCount: newInt64(4380),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Values: map[string]interface{}{
										"1.0":  0.0,
										"5.0":  4.0,
										"25.0": 24.0,
										"50.0": 48.81132075471698,
										"75.0": 75.0,
										"95.0": 95.0,
										"99.0": 99.0,
									},
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						"nairobi": {
							Key:      "nairobi",
							DocCount: newInt64(4380),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Values: map[string]interface{}{
										"1.0":  1.0,
										"5.0":  5.0,
										"25.0": 25.0,
										"50.0": 50.0,
										"75.0": 75.0,
										"95.0": 95.0,
										"99.0": 100.0,
									},
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
					},
					SubAggregations: aggregationsMap{},
					OtherValues:     map[string]interface{}{},
				},
			},
		},
		{
			Aggregations: aggregationsMap{
				"host": {
					Buckets: bucketsMap{
						"helsinki": {
							Key:      "helsinki",
							DocCount: newInt64(5022),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Values: map[string]interface{}{
										"1.0":  0.0,
										"5.0":  4.0,
										"25.0": 23.953488372093023,
										"50.0": 49.0,
										"75.0": 75.0,
										"95.0": 95.0,
										"99.0": 99.0,
									},
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						"nairobi": {
							Key:      "nairobi",
							DocCount: newInt64(5022),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Values: map[string]interface{}{
										"1.0":  1.0,
										"5.0":  5.0,
										"25.0": 25.0,
										"50.0": 50.0,
										"75.0": 75.0,
										"95.0": 95.0,
										"99.0": 99.13999999999987,
									},
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						"madrid": {
							Key:      "madrid",
							DocCount: newInt64(5022),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Values: map[string]interface{}{
										"1.0":  0.0,
										"5.0":  4.0,
										"25.0": 24.0,
										"50.0": 50.10666666666667,
										"75.0": 75.87878787878788,
										"95.0": 95.0,
										"99.0": 100.0,
									},
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
						"lisbon": {
							Key:      "lisbon",
							DocCount: newInt64(5022),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Values: map[string]interface{}{
										"1.0":  0.0,
										"5.0":  4.0,
										"25.0": 24.0,
										"50.0": 50.0,
										"75.0": 75.0,
										"95.0": 95.0,
										"99.0": 100.0,
									},
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
							},
						},
					},
					SubAggregations: aggregationsMap{},
					OtherValues:     map[string]interface{}{},
				},
			},
		},
	}
	return getAggregationTestContexts(filePaths, expected)
}

func getMultipleMetricAggregationTestContexts() ([]*testContext, *errors.Error) {
	filePaths := []string{
		"testdata/multi_metric_aggs_with_terms_aggs.json",
		"testdata/multi_metric_aggs_with_filters_aggs.json",
	}

	expected := []HTTPResponse{
		{
			Aggregations: aggregationsMap{
				"host": {
					Buckets: bucketsMap{
						"nairobi": {
							Key:      "nairobi",
							DocCount: newInt64(5134),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Values: map[string]interface{}{
										"1.0":  1.0,
										"5.0":  5.0,
										"25.0": 25.0,
										"50.0": 50.0,
										"75.0": 75.0,
										"95.0": 95.0,
										"99.0": 99.07999999999993,
									},
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
								"metric_agg_2": {
									Value:           50.14530580444098,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
								"metric_agg_3": {
									SubAggregations: aggregationsMap{},
									OtherValues: map[string]interface{}{
										"count":          5134.0,
										"min":            0.0,
										"max":            100.0,
										"avg":            50.14530580444098,
										"sum":            257446.0,
										"sum_of_squares": 1.7184548E7,
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
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Values: map[string]interface{}{
										"1.0":  0.0,
										"5.0":  4.0,
										"25.0": 24.0,
										"50.0": 50.294871794871796,
										"75.0": 75.98039215686275,
										"95.0": 95.0,
										"99.0": 100.0,
									},
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
								"metric_agg_2": {
									Value:           50.03486560186989,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
								"metric_agg_3": {
									SubAggregations: aggregationsMap{},
									OtherValues: map[string]interface{}{
										"count":          5134.0,
										"min":            0.0,
										"max":            100.0,
										"avg":            50.03486560186989,
										"sum":            256879.0,
										"sum_of_squares": 1.7288541E7,
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
						"lisbon": {
							Key:      "lisbon",
							DocCount: newInt64(5134),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Values: map[string]interface{}{
										"1.0":  0.0,
										"5.0":  4.0,
										"25.0": 24.0,
										"50.0": 49.945454545454545,
										"75.0": 74.87301587301587,
										"95.0": 95.0,
										"99.0": 100.0,
									},
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
								"metric_agg_2": {
									Value:           49.577522399688355,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
								"metric_agg_3": {
									SubAggregations: aggregationsMap{},
									OtherValues: map[string]interface{}{
										"count":          5134.0,
										"min":            0.0,
										"max":            100.0,
										"avg":            49.577522399688355,
										"sum":            254531.0,
										"sum_of_squares": 1.7057901E7,
										"variance":       864.6055017695604,
										"std_deviation":  29.40417490373706,
										"std_deviation_bounds": map[string]interface{}{
											"upper": 108.38587220716248,
											"lower": -9.230827407785767,
										},
									},
								},
							},
						},
						"helsinki": {
							Key:      "helsinki",
							DocCount: newInt64(5134),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Values: map[string]interface{}{
										"1.0":  0.0,
										"5.0":  4.0,
										"25.0": 23.977272727272727,
										"50.0": 49.0,
										"75.0": 75.0,
										"95.0": 95.0,
										"99.0": 99.0,
									},
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
								"metric_agg_2": {
									Value:           49.273081417997666,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
								"metric_agg_3": {
									SubAggregations: aggregationsMap{},
									OtherValues: map[string]interface{}{
										"count":          5134.0,
										"min":            0.0,
										"max":            100.0,
										"avg":            49.273081417997666,
										"sum":            252968.0,
										"sum_of_squares": 1.6862378E7,
										"variance":       856.6157265001883,
										"std_deviation":  29.267998334361515,
										"std_deviation_bounds": map[string]interface{}{
											"upper": 107.80907808672069,
											"lower": -9.262915250725364,
										},
									},
								},
							},
						},
					},
					SubAggregations: aggregationsMap{},
					OtherValues:     map[string]interface{}{},
				},
			},
		},
		{
			Aggregations: aggregationsMap{
				"host": {
					Buckets: bucketsMap{
						"nairobi": {
							Key:      "nairobi",
							DocCount: newInt64(5366),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Values: map[string]interface{}{
										"1.0":  1.0,
										"5.0":  5.0,
										"25.0": 25.0,
										"50.0": 50.0,
										"75.0": 75.0,
										"95.0": 95.0,
										"99.0": 99.0,
									},
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
								"metric_agg_2": {
									Value:           50.084047707789786,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
								"metric_agg_3": {
									SubAggregations: aggregationsMap{},
									OtherValues: map[string]interface{}{
										"count":          5366.0,
										"min":            0.0,
										"max":            100.0,
										"avg":            50.084047707789786,
										"sum":            268751.0,
										"sum_of_squares": 1.7938575E7,
										"variance":       834.5950604703294,
										"std_deviation":  28.889358948760517,
										"std_deviation_bounds": map[string]interface{}{
											"upper": 107.86276560531081,
											"lower": -7.694670189731248,
										},
									},
								},
							},
						},
						"helsinki": {
							Key:      "helsinki",
							DocCount: newInt64(5366),
							SubAggregations: aggregationsMap{
								"metric_agg_1": {
									Values: map[string]interface{}{
										"1.0":  0.0,
										"5.0":  4.0,
										"25.0": 24.0,
										"50.0": 49.0,
										"75.0": 75.0,
										"95.0": 95.0,
										"99.0": 99.0,
									},
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
								"metric_agg_2": {
									Value:           49.27953783078643,
									SubAggregations: aggregationsMap{},
									OtherValues:     map[string]interface{}{},
								},
								"metric_agg_3": {
									SubAggregations: aggregationsMap{},
									OtherValues: map[string]interface{}{
										"count":          5366.0,
										"min":            0.0,
										"max":            100.0,
										"avg":            49.27953783078643,
										"sum":            264434.0,
										"sum_of_squares": 1.7629144E7,
										"variance":       856.8689327718637,
										"std_deviation":  29.27232366539875,
										"std_deviation_bounds": map[string]interface{}{
											"upper": 107.82418516158393,
											"lower": -9.265109500011071,
										},
									},
								},
							},
						},
					},
					SubAggregations: aggregationsMap{},
					OtherValues:     map[string]interface{}{},
				},
			},
		},
	}

	return getAggregationTestContexts(filePaths, expected)
}

func getAggregationTestContexts(responseFilePaths []string, expected []HTTPResponse) ([]*testContext, *errors.Error) {

	out := make([]*testContext, len(responseFilePaths))

	for i, fp := range responseFilePaths {
		res, err := loadJSON(fp)

		if err != nil {
			return nil, errors.Errorf("failed loading %s", fp)
		}

		var parsedRes HTTPResponse
		if err := json.Unmarshal(res, &parsedRes); err != nil {
			return nil, errors.Errorf("Error processing HTTP response: %s", err)
		}

		out[i] = &testContext{
			rawFile:  fp,
			actual:   parsedRes,
			expected: expected[i],
		}
	}

	return out, nil
}

func loadJSON(filePath string) ([]byte, error) {
	return ioutil.ReadFile(filePath)
}

func newInt64(num int64) *int64 {
	out := new(int64)
	*out = num
	return out
}

func newFloat64(num float64) *float64 {
	out := new(float64)
	*out = num
	return out
}
