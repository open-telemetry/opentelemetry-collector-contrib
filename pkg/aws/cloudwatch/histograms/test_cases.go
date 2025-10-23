// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package histograms // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/aws/cloudwatch/histograms"

import (
	"math"
)

type HistogramTestCase struct {
	Name     string
	Input    HistogramInput
	Expected ExpectedMetrics
}

type HistogramInput struct {
	Count      uint64
	Sum        float64
	Min        *float64
	Max        *float64
	Boundaries []float64
	Counts     []uint64
	Attributes map[string]string
}

type PercentileRange struct {
	Low  float64
	High float64
}
type ExpectedMetrics struct {
	Count            uint64
	Sum              float64
	Average          float64
	Min              *float64
	Max              *float64
	PercentileRanges map[float64]PercentileRange
}

func TestCases() []HistogramTestCase {
	// Create large bucket arrays with 11 items per bucket
	boundaries125 := make([]float64, 125)
	counts125 := make([]uint64, 126)
	for i := 0; i < 125; i++ {
		boundaries125[i] = float64(i+1) * 10
		counts125[i] = 11
	}
	counts125[125] = 11

	boundaries175 := make([]float64, 175)
	counts175 := make([]uint64, 176)
	for i := 0; i < 175; i++ {
		boundaries175[i] = float64(i+1) * 10
		counts175[i] = 11
	}
	counts175[175] = 11

	boundaries225 := make([]float64, 225)
	counts225 := make([]uint64, 226)
	for i := 0; i < 225; i++ {
		boundaries225[i] = float64(i+1) * 10
		counts225[i] = 11
	}
	counts225[225] = 11

	boundaries325 := make([]float64, 325)
	counts325 := make([]uint64, 326)
	for i := 0; i < 325; i++ {
		boundaries325[i] = float64(i+1) * 10
		counts325[i] = 11
	}
	counts325[325] = 11

	return []HistogramTestCase{
		{
			Name: "Basic Histogram",
			Input: HistogramInput{
				Count:      101,
				Sum:        6000,
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{21, 31, 25, 15, 7, 2},
				Attributes: map[string]string{"service.name": "payment-service"},
			},
			Expected: ExpectedMetrics{
				Count:   101,
				Sum:     6000,
				Average: 59.41,
				Min:     ptr(10.0),
				Max:     ptr(200.0),
				PercentileRanges: map[float64]PercentileRange{
					0.01: {Low: 10.0, High: 25.0},
					0.1:  {Low: 10.0, High: 25.0},
					0.25: {Low: 25.0, High: 50.0},
					0.5:  {Low: 25.0, High: 50.0},
					0.75: {Low: 50.0, High: 75.0},
					0.9:  {Low: 75.0, High: 100.0},
					0.99: {Low: 150.0, High: 200.0},
				},
			},
		},
		{
			Name: "Tail Heavy Histogram",
			Input: HistogramInput{
				Count:      1010,
				Sum:        145000,
				Min:        ptr(10.0),
				Max:        ptr(151.0),
				Boundaries: []float64{25, 50, 75, 100, 125, 150, 151, 200},
				Counts:     []uint64{3, 7, 15, 25, 50, 800, 100, 10, 0},
				Attributes: map[string]string{"service.name": "payment-service"},
			},
			Expected: ExpectedMetrics{
				Count:   1010,
				Sum:     145000,
				Average: 143.56,
				Min:     ptr(10.0),
				Max:     ptr(151.0),
				PercentileRanges: map[float64]PercentileRange{
					0.01: {Low: 25.0, High: 50.0},
					0.1:  {Low: 125.0, High: 150.0},
					0.25: {Low: 125.0, High: 150.0},
					0.5:  {Low: 125.0, High: 150.0},
					0.75: {Low: 125.0, High: 150.0},
					0.9:  {Low: 150.0, High: 151.0},
					0.99: {Low: 150.0, High: 151.0},
				},
			},
		},
		{
			Name: "Single Bucket",
			Input: HistogramInput{
				Count:      51,
				Sum:        1000,
				Min:        ptr(5.0),
				Max:        ptr(75.0),
				Boundaries: []float64{},
				Counts:     []uint64{51},
				Attributes: map[string]string{"service.name": "auth-service"},
			},
			Expected: ExpectedMetrics{
				Count:   51,
				Sum:     1000,
				Average: 19.61,
				Min:     ptr(5.0),
				Max:     ptr(75.0),
				PercentileRanges: map[float64]PercentileRange{
					0.01: {Low: 5.0, High: 75.0},
					0.1:  {Low: 5.0, High: 75.0},
					0.25: {Low: 5.0, High: 75.0},
					0.5:  {Low: 5.0, High: 75.0},
					0.75: {Low: 5.0, High: 75.0},
					0.9:  {Low: 5.0, High: 75.0},
					0.99: {Low: 5.0, High: 75.0},
				},
			},
		},
		{
			Name: "Two Buckets",
			Input: HistogramInput{
				Count:      31,
				Sum:        150,
				Min:        ptr(1.0),
				Max:        ptr(10.0),
				Boundaries: []float64{5},
				Counts:     []uint64{21, 10},
				Attributes: map[string]string{"service.name": "database"},
			},
			Expected: ExpectedMetrics{
				Count:   31,
				Sum:     150,
				Average: 4.84,
				Min:     ptr(1.0),
				Max:     ptr(10.0),
				PercentileRanges: map[float64]PercentileRange{
					0.01: {Low: 1.0, High: 5.0},
					0.1:  {Low: 1.0, High: 5.0},
					0.25: {Low: 1.0, High: 5.0},
					0.5:  {Low: 1.0, High: 5.0},
					0.75: {Low: 5.0, High: 10.0},
					0.9:  {Low: 5.0, High: 10.0},
					0.99: {Low: 5.0, High: 10.0},
				},
			},
		},
		{
			Name: "Zero Counts and Sparse Data",
			Input: HistogramInput{
				Count:      101,
				Sum:        25000,
				Min:        ptr(0.0),
				Max:        ptr(1500.0),
				Boundaries: []float64{10, 50, 100, 500, 1000},
				Counts:     []uint64{51, 0, 0, 39, 0, 11},
				Attributes: map[string]string{"service.name": "cache-service"},
			},
			Expected: ExpectedMetrics{
				Count:   101,
				Sum:     25000,
				Average: 247.52,
				Min:     ptr(0.0),
				Max:     ptr(1500.0),
				PercentileRanges: map[float64]PercentileRange{
					0.01: {Low: 0.0, High: 10.0},
					0.1:  {Low: 0.0, High: 10.0},
					0.25: {Low: 0.0, High: 10.0},
					0.5:  {Low: 0.0, High: 10.0},
					0.75: {Low: 100.0, High: 500.0},
					0.9:  {Low: 1000.0, High: 1500.0},
					0.99: {Low: 1000.0, High: 1500.0},
				},
			},
		},
		{
			Name: "Large Numbers",
			Input: HistogramInput{
				Count:      1001,
				Sum:        100000000000,
				Min:        ptr(100000.0),
				Max:        ptr(1000000000.0),
				Boundaries: []float64{1000000, 10000000, 50000000, 100000000, 500000000},
				Counts:     []uint64{201, 301, 249, 150, 50, 50},
				Attributes: map[string]string{"service.name": "batch-processor"},
			},
			Expected: ExpectedMetrics{
				Count:   1001,
				Sum:     100000000000,
				Average: 99900099.90,
				Min:     ptr(100000.0),
				Max:     ptr(1000000000.0),
				PercentileRanges: map[float64]PercentileRange{
					0.01: {Low: 100000.0, High: 1000000.0},
					0.1:  {Low: 100000.0, High: 1000000.0},
					0.25: {Low: 1000000.0, High: 10000000.0},
					0.5:  {Low: 1000000.0, High: 10000000.0},
					0.75: {Low: 10000000.0, High: 50000000.0},
					0.9:  {Low: 50000000.0, High: 100000000.0},
					0.99: {Low: 500000000.0, High: 1000000000.0},
				},
			},
		},
		{
			Name: "Many Buckets",
			Input: HistogramInput{
				Count:      1124,
				Sum:        350000,
				Min:        ptr(0.5),
				Max:        ptr(1100.0),
				Boundaries: []float64{1, 5, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 200, 300, 400, 500, 600, 700, 800, 900, 1000},
				Counts:     []uint64{51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 53},
				Attributes: map[string]string{"service.name": "detailed-metrics"},
			},
			Expected: ExpectedMetrics{
				Count:   1124,
				Sum:     350000,
				Average: 311.39,
				Min:     ptr(0.5),
				Max:     ptr(1100.0),
				PercentileRanges: map[float64]PercentileRange{
					0.01: {Low: 0.5, High: 1.0},
					0.1:  {Low: 5.0, High: 10.0},
					0.25: {Low: 30.0, High: 40.0},
					0.5:  {Low: 90.0, High: 100.0},
					0.75: {Low: 500.0, High: 600.0},
					0.9:  {Low: 800.0, High: 900.0},
					0.99: {Low: 1000.0, High: 1100.0},
				},
			},
		},
		{
			Name: "Very Small Numbers",
			Input: HistogramInput{
				Count:      101,
				Sum:        0.00015,
				Min:        ptr(0.00000001),
				Max:        ptr(0.000006),
				Boundaries: []float64{0.0000001, 0.000001, 0.000002, 0.000003, 0.000004, 0.000005},
				Counts:     []uint64{11, 21, 29, 20, 15, 4, 1},
				Attributes: map[string]string{"service.name": "micro-timing"},
			},
			Expected: ExpectedMetrics{
				Count:   101,
				Sum:     0.00015,
				Average: 0.00000149,
				Min:     ptr(0.00000001),
				Max:     ptr(0.000006),
				PercentileRanges: map[float64]PercentileRange{
					0.01: {Low: 0.00000001, High: 0.0000001},
					0.1:  {Low: 0.00000001, High: 0.0000001},
					0.25: {Low: 0.0000001, High: 0.000001},
					0.5:  {Low: 0.000001, High: 0.000002},
					0.75: {Low: 0.000002, High: 0.000003},
					0.9:  {Low: 0.000003, High: 0.000004},
					0.99: {Low: 0.000004, High: 0.000005},
				},
			},
		},
		{
			Name: "Only Negative Boundaries",
			Input: HistogramInput{
				Count:      101,
				Sum:        -10000,
				Min:        ptr(-200.0),
				Max:        ptr(-10.0),
				Boundaries: []float64{-150, -100, -75, -50, -25},
				Counts:     []uint64{21, 31, 25, 15, 7, 2},
				Attributes: map[string]string{"service.name": "negative-service"},
			},
			Expected: ExpectedMetrics{
				Count:   101,
				Sum:     -10000,
				Average: -99.01,
				Min:     ptr(-200.0),
				Max:     ptr(-10.0),
				// Can't get percentiles for negatives
				PercentileRanges: map[float64]PercentileRange{},
			},
		},
		{
			Name: "Negative and Positive Boundaries",
			Input: HistogramInput{
				Count:      106,
				Sum:        0,
				Min:        ptr(-50.0),
				Max:        ptr(50.0),
				Boundaries: []float64{-30, -10, 10, 30},
				Counts:     []uint64{25, 26, 5, 25, 25},
				Attributes: map[string]string{"service.name": "temperature-service"},
			},
			Expected: ExpectedMetrics{
				Count:   106,
				Sum:     0,
				Average: 0.0,
				Min:     ptr(-50.0),
				Max:     ptr(50.0),
				// Can't get percentiles for negatives
				PercentileRanges: map[float64]PercentileRange{},
			},
		},

		{
			Name: "Positive boundaries but implied Negative Values",
			Input: HistogramInput{
				Count:      101,
				Sum:        200,
				Min:        ptr(-100.0),
				Max:        ptr(60.0),
				Boundaries: []float64{0, 10, 20, 30, 40, 50},
				Counts:     []uint64{61, 10, 10, 10, 5, 4, 1},
				Attributes: map[string]string{"service.name": "temperature-service"},
			},
			Expected: ExpectedMetrics{
				Count:   101,
				Sum:     200,
				Average: 1.98,
				Min:     ptr(-100.0),
				Max:     ptr(60.0),
				// Can't get percentiles for negatives
				PercentileRanges: map[float64]PercentileRange{},
			},
		},
		{
			Name: "No Min or Max",
			Input: HistogramInput{
				Count:      75,
				Sum:        3500,
				Min:        nil,
				Max:        nil,
				Boundaries: []float64{10, 50, 100, 200},
				Counts:     []uint64{15, 21, 24, 10, 5},
				Attributes: map[string]string{"service.name": "web-service"},
			},
			Expected: ExpectedMetrics{
				Count:   75,
				Sum:     3500,
				Average: 46.67,
				Min:     nil,
				Max:     nil,
				PercentileRanges: map[float64]PercentileRange{
					0.1:  {Low: math.Inf(-1), High: 10.0},
					0.25: {Low: 10.0, High: 50.0},
					0.5:  {Low: 50.0, High: 100.0},
					0.75: {Low: 50.0, High: 100.0},
					0.9:  {Low: 100.0, High: 200.0},
				},
			},
		},
		{
			Name: "Only Max Defined",
			Input: HistogramInput{
				Count:      101,
				Sum:        17500,
				Min:        nil,
				Max:        ptr(750.0),
				Boundaries: []float64{100, 200, 300, 400, 500},
				Counts:     []uint64{21, 31, 24, 15, 5, 5},
				Attributes: map[string]string{"service.name": "api-gateway"},
			},
			Expected: ExpectedMetrics{
				Count:   101,
				Sum:     17500,
				Average: 173.27,
				Min:     nil,
				Max:     ptr(750.0),
				PercentileRanges: map[float64]PercentileRange{
					0.1:  {Low: math.Inf(-1), High: 100.0},
					0.25: {Low: 100.0, High: 200.0},
					0.5:  {Low: 100.0, High: 200.0},
					0.75: {Low: 200.0, High: 300.0},
					0.9:  {Low: 300.0, High: 400.0},
				},
			},
		},
		{
			Name: "Only Min Defined",
			Input: HistogramInput{
				Count:      51,
				Sum:        4000,
				Min:        ptr(25.0),
				Max:        nil,
				Boundaries: []float64{50, 100, 150},
				Counts:     []uint64{11, 21, 14, 5},
				Attributes: map[string]string{"service.name": "queue-service"},
			},
			Expected: ExpectedMetrics{
				Count:   51,
				Sum:     4000,
				Average: 78.43,
				Min:     ptr(25.0),
				Max:     nil,
				PercentileRanges: map[float64]PercentileRange{
					0.1:  {Low: 25.0, High: 50.0},
					0.25: {Low: 50.0, High: 100.0},
					0.5:  {Low: 50.0, High: 100.0},
					0.75: {Low: 100.0, High: 150.0},
					0.9:  {Low: 100.0, High: 150.0},
				},
			},
		},
		{
			Name: "No Min/Max with Single Value",
			Input: HistogramInput{
				Count:      1,
				Sum:        100,
				Min:        nil,
				Max:        nil,
				Boundaries: []float64{50, 150},
				Counts:     []uint64{0, 1, 0},
				Attributes: map[string]string{"service.name": "singleton-service"},
			},
			Expected: ExpectedMetrics{
				Count:   1,
				Sum:     100,
				Average: 100.0,
				Min:     nil,
				Max:     nil,
				PercentileRanges: map[float64]PercentileRange{
					0.1:  {Low: 50.0, High: 150.0},
					0.25: {Low: 50.0, High: 150.0},
					0.5:  {Low: 50.0, High: 150.0},
					0.75: {Low: 50.0, High: 150.0},
					0.9:  {Low: 50.0, High: 150.0},
				},
			},
		},
		{
			Name: "Unbounded Histogram",
			Input: HistogramInput{
				Count:      75,
				Sum:        3500,
				Min:        nil,
				Max:        nil,
				Boundaries: []float64{},
				Counts:     []uint64{75},
				Attributes: map[string]string{"service.name": "unbounded-service"},
			},
			Expected: ExpectedMetrics{
				Count:   75,
				Sum:     3500,
				Average: 46.67,
				Min:     nil,
				Max:     nil,
				PercentileRanges: map[float64]PercentileRange{
					0.1:  {Low: math.Inf(-1), High: math.Inf(1)},
					0.25: {Low: math.Inf(-1), High: math.Inf(1)},
					0.5:  {Low: math.Inf(-1), High: math.Inf(1)},
					0.75: {Low: math.Inf(-1), High: math.Inf(1)},
					0.9:  {Low: math.Inf(-1), High: math.Inf(1)},
				},
			},
		},
		{
			// mimics Kubernetes request_wait_duration_seconds bucket definition
			// https://github.com/kubernetes/kubernetes/blob/476325f6e5febeb1c5beb673aa01474518310811/test/instrumentation/testdata/stable-metrics-list.yaml#L111-L134
			Name: "Cumulative bucket starts at 0",
			Input: HistogramInput{
				Count:      3181,
				Sum:        1100,
				Min:        nil,
				Max:        nil,
				Boundaries: []float64{0, 0.005, 0.02, 0.1, 0.2, 0.5, 1, 2, 5, 10, 15, 30},
				Counts:     []uint64{100, 150, 200, 1000, 800, 650, 200, 40, 20, 10, 8, 2, 1},
				Attributes: map[string]string{"service.name": "unbounded-service"},
			},
			Expected: ExpectedMetrics{
				Count:   3181,
				Sum:     1100,
				Average: 0.35,
				Min:     nil,
				Max:     nil,
				PercentileRanges: map[float64]PercentileRange{
					0.1:  {Low: 0.005, High: 0.02},
					0.25: {Low: 0.02, High: 0.1},
					0.5:  {Low: 0.1, High: 0.2},
					0.75: {Low: 0.2, High: 0.5},
					0.9:  {Low: 0.2, High: 0.5},
				},
			},
		},
		// >100 buckets will be used for testing request splitting in PMD path
		{
			Name: "126 Buckets",
			Input: HistogramInput{
				Count:      1386, // 126 buckets * 11 items each
				Sum:        870555,
				Min:        ptr(5.0),
				Max:        ptr(1300.0),
				Boundaries: boundaries125,
				Counts:     counts125,
				Attributes: map[string]string{"service.name": "many-buckets-125"},
			},
			Expected: ExpectedMetrics{
				Count:   1386,
				Sum:     870555,
				Average: 628.11,
				Min:     ptr(5.0),
				Max:     ptr(1300.0),
				PercentileRanges: map[float64]PercentileRange{
					0.1:  {Low: 120.0, High: 130.0},
					0.25: {Low: 310.0, High: 320.0},
					0.5:  {Low: 620.0, High: 630.0},
					0.75: {Low: 940.0, High: 950.0},
					0.9:  {Low: 1130.0, High: 1140.0},
				},
			},
		},
		// >150 buckets will be used for testing request splitting in EMF path
		{
			Name: "176 Buckets",
			Input: HistogramInput{
				Count:      1936, // 176 buckets * 11 items each
				Sum:        1697000,
				Min:        ptr(5.0),
				Max:        ptr(1800.0),
				Boundaries: boundaries175,
				Counts:     counts175,
				Attributes: map[string]string{"service.name": "many-buckets-175"},
			},
			Expected: ExpectedMetrics{
				Count:   1936,
				Sum:     1697000,
				Average: 876.55,
				Min:     ptr(5.0),
				Max:     ptr(1800.0),
				PercentileRanges: map[float64]PercentileRange{
					0.1:  {Low: 170.0, High: 180.0},
					0.25: {Low: 430.0, High: 440.0},
					0.5:  {Low: 870.0, High: 880.0},
					0.75: {Low: 1310.0, High: 1320.0},
					0.9:  {Low: 1580.0, High: 1590.0},
				},
			},
		},
		// PMD should split into 3 requests
		// EMF should split into 2 requests
		{
			Name: "225 Buckets",
			Input: HistogramInput{
				Count:      2486, // 226 buckets * 11 items each
				Sum:        2803750,
				Min:        ptr(5.0),
				Max:        ptr(2300.0),
				Boundaries: boundaries225,
				Counts:     counts225,
				Attributes: map[string]string{"service.name": "many-buckets-225"},
			},
			Expected: ExpectedMetrics{
				Count:   2486,
				Sum:     2803750,
				Average: 1127.82,
				Min:     ptr(5.0),
				Max:     ptr(2300.0),
				PercentileRanges: map[float64]PercentileRange{
					0.1:  {Low: 220.0, High: 230.0},
					0.25: {Low: 560.0, High: 570.0},
					0.5:  {Low: 1120.0, High: 1130.0},
					0.75: {Low: 1690.0, High: 1700.0},
					0.9:  {Low: 2030.0, High: 2040.0},
				},
			},
		},
		// PMD should split into 4 requests
		// EMF should split into 3 requests
		{
			Name: "325 Buckets",
			Input: HistogramInput{
				Count:      3586, // 326 buckets * 11 items each
				Sum:        5830500,
				Min:        ptr(5.0),
				Max:        ptr(3300.0),
				Boundaries: boundaries325,
				Counts:     counts325,
				Attributes: map[string]string{"service.name": "many-buckets-325"},
			},
			Expected: ExpectedMetrics{
				Count:   3586,
				Sum:     5830500,
				Average: 1625.91,
				Min:     ptr(5.0),
				Max:     ptr(3300.0),
				PercentileRanges: map[float64]PercentileRange{
					0.1:  {Low: 320.0, High: 330.0},
					0.25: {Low: 810.0, High: 820.0},
					0.5:  {Low: 1620.0, High: 1630.0},
					0.75: {Low: 2440.0, High: 2450.0},
					0.9:  {Low: 2930.0, High: 2940.0},
				},
			},
		},
	}
}

func InvalidTestCases() []HistogramTestCase {
	return []HistogramTestCase{
		{
			Name: "Boundaries Not Ascending",
			Input: HistogramInput{
				Count:      100,
				Sum:        5000,
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 40, 100, 150}, // 40 < 50
				Counts:     []uint64{20, 30, 25, 15, 8, 2},
				Attributes: map[string]string{"service.name": "invalid-boundaries"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Counts Length Mismatch",
			Input: HistogramInput{
				Count:      100,
				Sum:        5000,
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100},
				Counts:     []uint64{20, 30, 25, 15, 8, 2}, // Should be 5 counts for 4 boundaries
				Attributes: map[string]string{"service.name": "wrong-counts"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Total Count Mismatch",
			Input: HistogramInput{
				Count:      90, // Doesn't match sum of counts (100)
				Sum:        5000,
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2},
				Attributes: map[string]string{"service.name": "count-mismatch"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Min Greater Than First Boundary",
			Input: HistogramInput{
				Count:      100,
				Sum:        5000,
				Min:        ptr(30.0), // Greater than first boundary (25)
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2}, // Has counts in first bucket
				Attributes: map[string]string{"service.name": "invalid-min"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Max Less Than Last Boundary",
			Input: HistogramInput{
				Count:      100,
				Sum:        5000,
				Min:        ptr(10.0),
				Max:        ptr(140.0), // Less than last boundary (150)
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2}, // Has counts in overflow bucket
				Attributes: map[string]string{"service.name": "invalid-max"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Sum Too Small",
			Input: HistogramInput{
				Count:      100,
				Sum:        100, // Too small given the boundaries and counts
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2},
				Attributes: map[string]string{"service.name": "small-sum"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Sum Too Large",
			Input: HistogramInput{
				Count:      100,
				Sum:        1000000, // Too large given the boundaries and counts
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2},
				Attributes: map[string]string{"service.name": "large-sum"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Min in Second Bucket But Sum Too Low",
			Input: HistogramInput{
				Count:      100,
				Sum:        2000,      // This sum is too low given min is in second bucket
				Min:        ptr(60.0), // Min falls in second bucket (50,75]
				Max:        ptr(200.0),
				Boundaries: []float64{50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 10}, // 30 values must be at least 60 each in second bucket
				Attributes: map[string]string{"service.name": "invalid-min-bucket"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Max in Second-to-Last Bucket But Sum Too High",
			Input: HistogramInput{
				Count:      100,
				Sum:        10000, // This sum is too high given max is in second-to-last bucket
				Min:        ptr(10.0),
				Max:        ptr(90.0), // Max falls in second-to-last bucket (75,100]
				Boundaries: []float64{50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 10}, // No value can exceed 90
				Attributes: map[string]string{"service.name": "invalid-max-bucket"},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "NaN Sum",
			Input: HistogramInput{
				Count:      100,
				Sum:        math.NaN(),
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Inf Sum",
			Input: HistogramInput{
				Count:      100,
				Sum:        math.Inf(1),
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "NaN Boundary",
			Input: HistogramInput{
				Count:      100,
				Sum:        5000,
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, math.NaN(), 75, 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2},
			},
			Expected: ExpectedMetrics{},
		},
		{
			Name: "Inf Boundary",
			Input: HistogramInput{
				Count:      100,
				Sum:        5000,
				Min:        ptr(10.0),
				Max:        ptr(200.0),
				Boundaries: []float64{25, 50, math.Inf(1), 100, 150},
				Counts:     []uint64{20, 30, 25, 15, 8, 2},
			},
			Expected: ExpectedMetrics{},
		},
	}
}

func ptr(f float64) *float64 {
	return &f
}
