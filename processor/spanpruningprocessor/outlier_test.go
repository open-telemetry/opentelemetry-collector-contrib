// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestAnalyzeOutliers(t *testing.T) {
	ms := time.Millisecond

	defaultCfg := OutlierAnalysisConfig{
		IQRMultiplier:                  1.5,
		MinGroupSize:                   7,
		CorrelationMinOccurrence:       0.75,
		CorrelationMaxNormalOccurrence: 0.25,
		MaxCorrelatedAttributes:        5,
	}

	tests := []struct {
		name             string
		durations        []time.Duration
		attrs            []map[string]string
		cfg              OutlierAnalysisConfig
		wantMedian       time.Duration
		wantCorrelations int
		wantTopKey       string
		wantTopValue     string
	}{
		{
			name: "clear outliers with correlation",
			durations: []time.Duration{
				5 * ms, 6 * ms, 6 * ms, 7 * ms, 8 * ms, 9 * ms, 10 * ms, 11 * ms, // normal
				500 * ms, 600 * ms, // outliers
			},
			attrs: []map[string]string{
				{"db.cache_hit": "true"},
				{"db.cache_hit": "true"},
				{"db.cache_hit": "true"},
				{"db.cache_hit": "true"},
				{"db.cache_hit": "true"},
				{"db.cache_hit": "true"},
				{"db.cache_hit": "true"},
				{"db.cache_hit": "true"},
				{"db.cache_hit": "false"}, // outlier
				{"db.cache_hit": "false"}, // outlier
			},
			cfg:              defaultCfg,
			wantMedian:       (8*ms + 9*ms) / 2,
			wantCorrelations: 1,
			wantTopKey:       "db.cache_hit",
			wantTopValue:     "false",
		},
		{
			name: "no outliers",
			durations: []time.Duration{
				5 * ms, 6 * ms, 6 * ms, 7 * ms, 8 * ms, 9 * ms, 10 * ms,
			},
			attrs: []map[string]string{
				{"key": "a"},
				{"key": "b"},
				{"key": "c"},
				{"key": "d"},
				{"key": "e"},
				{"key": "f"},
				{"key": "g"},
			},
			cfg:              defaultCfg,
			wantMedian:       7 * ms,
			wantCorrelations: 0,
		},
		{
			name:       "group too small",
			durations:  []time.Duration{5 * ms, 100 * ms, 200 * ms},
			attrs:      []map[string]string{{"a": "1"}, {"a": "2"}, {"a": "3"}},
			cfg:        defaultCfg,
			wantMedian: 0, // nil result
		},
		{
			name: "all same duration - no outliers",
			durations: []time.Duration{
				10 * ms, 10 * ms, 10 * ms, 10 * ms, 10 * ms, 10 * ms, 10 * ms,
			},
			attrs: []map[string]string{
				{"a": "1"},
				{"a": "2"},
				{"a": "3"},
				{"a": "4"},
				{"a": "5"},
				{"a": "6"},
				{"a": "7"},
			},
			cfg:              defaultCfg,
			wantMedian:       10 * ms,
			wantCorrelations: 0,
		},
		{
			name: "outliers but no strong correlation",
			durations: []time.Duration{
				5 * ms, 6 * ms, 6 * ms, 7 * ms, 8 * ms,
				150 * ms, 200 * ms,
			},
			attrs: []map[string]string{
				{"shard": "1"},
				{"shard": "2"},
				{"shard": "3"},
				{"shard": "1"},
				{"shard": "2"},
				{"shard": "1"},
				{"shard": "2"}, // outliers have same distribution as normals
			},
			cfg:              defaultCfg,
			wantMedian:       7 * ms,
			wantCorrelations: 0, // no strong correlation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := makeNodesWithAttrs(tt.durations, tt.attrs)
			result := analyzeOutliers(nodes, tt.cfg)

			if tt.wantMedian == 0 {
				require.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Equal(t, tt.wantMedian, result.median)
			assert.Len(t, result.correlations, tt.wantCorrelations)

			if tt.wantCorrelations > 0 {
				assert.Equal(t, tt.wantTopKey, result.correlations[0].key)
				assert.Equal(t, tt.wantTopValue, result.correlations[0].value)
			}
		})
	}
}

func TestAnalyzeOutliers_IQRZeroDetectsSpikeOutlier(t *testing.T) {
	ms := time.Millisecond

	cfg := OutlierAnalysisConfig{
		IQRMultiplier:                  1.5,
		MinGroupSize:                   7,
		CorrelationMinOccurrence:       0.75,
		CorrelationMaxNormalOccurrence: 0.25,
		MaxCorrelatedAttributes:        5,
	}

	durations := []time.Duration{10 * ms, 10 * ms, 10 * ms, 10 * ms, 10 * ms, 10 * ms, 1000 * ms}
	attrs := []map[string]string{
		{"cache_hit": "true"},
		{"cache_hit": "true"},
		{"cache_hit": "true"},
		{"cache_hit": "true"},
		{"cache_hit": "true"},
		{"cache_hit": "true"},
		{"cache_hit": "false"}, // spike outlier
	}

	nodes := makeNodesWithAttrs(durations, attrs)
	result := analyzeOutliers(nodes, cfg)

	require.NotNil(t, result)
	assert.Equal(t, 10*ms, result.median)
	require.True(t, result.hasOutliers)
	require.Len(t, result.outlierIndices, 1)
	assert.Equal(t, 6, result.outlierIndices[0])
	require.Len(t, result.normalIndices, 6)

	// Ensure correlation is computed (outliers=1, normals=6)
	require.NotEmpty(t, result.correlations)
	assert.Equal(t, "cache_hit", result.correlations[0].key)
	assert.Equal(t, "false", result.correlations[0].value)
}

func TestAnalyzeOutliers_AllOutliersStillReturnsIndices(t *testing.T) {
	ms := time.Millisecond

	// Negative multipliers are rejected by config validation, but analyzeOutliers
	// should still behave consistently if called with a malformed config.
	// With min_outlier_threshold_percent=0, anything above median is an outlier.
	cfg := OutlierAnalysisConfig{
		IQRMultiplier:                  -100,
		MinGroupSize:                   7,
		CorrelationMinOccurrence:       0.75,
		CorrelationMaxNormalOccurrence: 0.25,
		MaxCorrelatedAttributes:        5,
		MinOutlierThresholdPercent:     0, // Anything above median is outlier
	}

	durations := []time.Duration{5 * ms, 6 * ms, 7 * ms, 8 * ms, 9 * ms, 10 * ms, 11 * ms}
	nodes := makeNodesWithAttrs(durations, nil)
	result := analyzeOutliers(nodes, cfg)

	// With median=8ms and threshold=8ms (min_outlier_threshold_percent=0),
	// spans 9ms, 10ms, 11ms are outliers (3 outliers, 4 normals)
	require.NotNil(t, result)
	require.True(t, result.hasOutliers)
	assert.Len(t, result.outlierIndices, 3)
	assert.Len(t, result.normalIndices, 4)
}

func TestFormatCorrelations(t *testing.T) {
	tests := []struct {
		name         string
		correlations []attributeCorrelation
		want         string
	}{
		{
			name:         "empty",
			correlations: nil,
			want:         "",
		},
		{
			name: "single",
			correlations: []attributeCorrelation{
				{key: "db.cache_hit", value: "false", outlierOccurrence: 1.0, normalOccurrence: 0.0},
			},
			want: "db.cache_hit=false(100%/0%)",
		},
		{
			name: "multiple",
			correlations: []attributeCorrelation{
				{key: "db.cache_hit", value: "false", outlierOccurrence: 1.0, normalOccurrence: 0.0},
				{key: "db.shard", value: "7", outlierOccurrence: 0.8, normalOccurrence: 0.1},
			},
			want: "db.cache_hit=false(100%/0%), db.shard=7(80%/10%)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCorrelations(tt.correlations)
			assert.Equal(t, tt.want, got)
		})
	}
}

// makeNodesWithAttrs creates spanNodes with specified durations and attributes.
func makeNodesWithAttrs(durations []time.Duration, attrs []map[string]string) []*spanNode {
	nodes := make([]*spanNode, len(durations))
	baseTime := pcommon.NewTimestampFromTime(time.Now())

	for i, dur := range durations {
		span := ptrace.NewSpan()
		span.SetName("test")
		span.SetStartTimestamp(baseTime)
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(baseTime.AsTime().Add(dur)))

		if i < len(attrs) {
			for k, v := range attrs[i] {
				span.Attributes().PutStr(k, v)
			}
		}

		nodes[i] = &spanNode{span: span}
	}
	return nodes
}

func TestFilterOutlierNodes(t *testing.T) {
	ms := time.Millisecond

	tests := []struct {
		name                   string
		durations              []time.Duration
		attrs                  []map[string]string
		cfg                    OutlierAnalysisConfig
		wantNormalCount        int
		wantOutlierCount       int
		wantPreservedDurations []time.Duration // Most extreme first
	}{
		{
			name: "preserves top 2 outliers",
			durations: []time.Duration{
				5 * ms, 6 * ms, 7 * ms, 8 * ms, 9 * ms, 10 * ms, 11 * ms, 12 * ms, 13 * ms, 14 * ms, // normal (10 spans)
				500 * ms, 600 * ms, // outliers (2 spans) - ~17% of data, well outside normal range
			},
			attrs: []map[string]string{
				{"key": "a"},
				{"key": "b"},
				{"key": "c"},
				{"key": "d"},
				{"key": "e"},
				{"key": "f"},
				{"key": "g"},
				{"key": "h"},
				{"key": "i"},
				{"key": "j"},
				{"key": "k"},
				{"key": "l"},
			},
			cfg: OutlierAnalysisConfig{
				PreserveOutliers:               true,
				MaxPreservedOutliers:           1,
				IQRMultiplier:                  1.5,
				MinGroupSize:                   7,
				CorrelationMinOccurrence:       0.5,
				CorrelationMaxNormalOccurrence: 0.5,
				MaxCorrelatedAttributes:        5,
			},
			wantNormalCount:        11, // 10 normal + 1 outlier not preserved
			wantOutlierCount:       1,
			wantPreservedDurations: []time.Duration{600 * ms},
		},
		{
			name: "preserve disabled returns all as normal",
			durations: []time.Duration{
				5 * ms, 6 * ms, 7 * ms, 8 * ms, 9 * ms, 10 * ms, 11 * ms, 12 * ms, 13 * ms, 14 * ms, 500 * ms,
			},
			attrs: []map[string]string{
				{"key": "a"},
				{"key": "b"},
				{"key": "c"},
				{"key": "d"},
				{"key": "e"},
				{"key": "f"},
				{"key": "g"},
				{"key": "h"},
				{"key": "i"},
				{"key": "j"},
				{"key": "k"},
			},
			cfg: OutlierAnalysisConfig{
				PreserveOutliers:               false,
				MinGroupSize:                   7,
				IQRMultiplier:                  1.5,
				CorrelationMinOccurrence:       0.5,
				CorrelationMaxNormalOccurrence: 0.5,
				MaxCorrelatedAttributes:        5,
			},
			wantNormalCount:  11,
			wantOutlierCount: 0,
		},
		{
			name: "preserves all outliers when max is 0",
			durations: []time.Duration{
				5 * ms, 6 * ms, 7 * ms, 8 * ms, 9 * ms, 10 * ms, 11 * ms, 12 * ms, 13 * ms, 14 * ms,
				500 * ms, 600 * ms,
			},
			attrs: []map[string]string{
				{"key": "a"},
				{"key": "b"},
				{"key": "c"},
				{"key": "d"},
				{"key": "e"},
				{"key": "f"},
				{"key": "g"},
				{"key": "h"},
				{"key": "i"},
				{"key": "j"},
				{"key": "k"},
				{"key": "l"},
			},
			cfg: OutlierAnalysisConfig{
				PreserveOutliers:               true,
				MaxPreservedOutliers:           0, // 0 = preserve all
				IQRMultiplier:                  1.5,
				MinGroupSize:                   7,
				CorrelationMinOccurrence:       0.5,
				CorrelationMaxNormalOccurrence: 0.5,
				MaxCorrelatedAttributes:        5,
			},
			wantNormalCount:        10,
			wantOutlierCount:       2,
			wantPreservedDurations: []time.Duration{600 * ms, 500 * ms},
		},
		{
			name: "skip preservation without correlation",
			durations: []time.Duration{
				5 * ms, 6 * ms, 7 * ms, 8 * ms, 9 * ms, 10 * ms, 11 * ms, 12 * ms, 13 * ms, 14 * ms,
				500 * ms, 600 * ms,
			},
			attrs: []map[string]string{
				// No distinguishing attributes - varied values
				{"shard": "1"},
				{"shard": "2"},
				{"shard": "3"},
				{"shard": "1"},
				{"shard": "2"},
				{"shard": "3"},
				{"shard": "1"},
				{"shard": "2"},
				{"shard": "3"},
				{"shard": "1"},
				{"shard": "2"},
				{"shard": "3"},
			},
			cfg: OutlierAnalysisConfig{
				PreserveOutliers:               true,
				PreserveOnlyWithCorrelation:    true,
				MaxPreservedOutliers:           3,
				IQRMultiplier:                  1.5,
				MinGroupSize:                   7,
				CorrelationMinOccurrence:       0.75,
				CorrelationMaxNormalOccurrence: 0.25,
				MaxCorrelatedAttributes:        5,
			},
			wantNormalCount:  12, // All returned as normal (no correlation found)
			wantOutlierCount: 0,
		},
		{
			name: "preserves with correlation when required",
			durations: []time.Duration{
				5 * ms, 6 * ms, 7 * ms, 8 * ms, 9 * ms, 10 * ms, 11 * ms, 12 * ms, 13 * ms, 14 * ms,
				500 * ms, 600 * ms,
			},
			attrs: []map[string]string{
				{"cache": "hit"},
				{"cache": "hit"},
				{"cache": "hit"},
				{"cache": "hit"},
				{"cache": "hit"},
				{"cache": "hit"},
				{"cache": "hit"},
				{"cache": "hit"},
				{"cache": "hit"},
				{"cache": "hit"},
				{"cache": "miss"},
				{"cache": "miss"}, // outliers
			},
			cfg: OutlierAnalysisConfig{
				PreserveOutliers:               true,
				PreserveOnlyWithCorrelation:    true,
				MaxPreservedOutliers:           3,
				IQRMultiplier:                  1.5,
				MinGroupSize:                   7,
				CorrelationMinOccurrence:       0.75,
				CorrelationMaxNormalOccurrence: 0.25,
				MaxCorrelatedAttributes:        5,
			},
			wantNormalCount:        10,
			wantOutlierCount:       2,
			wantPreservedDurations: []time.Duration{600 * ms, 500 * ms},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := makeNodesWithAttrs(tt.durations, tt.attrs)
			analysis := analyzeOutliers(nodes, tt.cfg)
			normal, outliers := filterOutlierNodes(nodes, analysis, tt.cfg)

			assert.Len(t, normal, tt.wantNormalCount)
			assert.Len(t, outliers, tt.wantOutlierCount)

			if tt.wantPreservedDurations != nil {
				for i, want := range tt.wantPreservedDurations {
					got := getDuration(outliers[i])
					assert.Equal(t, want, got, "outlier %d duration", i)
				}
			}
		})
	}
}

func TestGetDuration(t *testing.T) {
	baseTime := pcommon.NewTimestampFromTime(time.Now())

	span := ptrace.NewSpan()
	span.SetStartTimestamp(baseTime)
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(baseTime.AsTime().Add(100 * time.Millisecond)))

	node := &spanNode{span: span}
	dur := getDuration(node)

	assert.Equal(t, 100*time.Millisecond, dur)
}

// runDetector exercises a detector through the same preparation
// analyzeOutliers performs: sort, take the raw median and minimum threshold,
// apply the configured transform, then classify. Spans are indexed in the order
// they are given.
func runDetector(t *testing.T, durations []time.Duration, method OutlierMethod, multiplier, minThresholdPercent float64, transform DurationTransform) ([]int, []int, time.Duration) {
	t.Helper()

	res := analyzeOutliers(makeNodesWithAttrs(durations, nil), OutlierAnalysisConfig{
		Method:                         method,
		DurationTransform:              transform,
		IQRMultiplier:                  multiplier,
		MADMultiplier:                  multiplier,
		MinGroupSize:                   4,
		MinOutlierThresholdPercent:     minThresholdPercent,
		CorrelationMinOccurrence:       0.5,
		CorrelationMaxNormalOccurrence: 0.5,
		MaxCorrelatedAttributes:        5,
	})
	require.NotNil(t, res)
	return res.outlierIndices, res.normalIndices, res.median
}

func TestDetectOutliersMAD_Basic(t *testing.T) {
	ms := time.Millisecond

	// Durations with clear outliers.
	durations := []time.Duration{
		5 * ms, 6 * ms, 7 * ms, 8 * ms, 9 * ms, 10 * ms, 11 * ms, 12 * ms,
		500 * ms, 600 * ms, // outliers
	}

	outlierIndices, normalIndices, median := runDetector(t, durations, OutlierMethodMAD, 3.0, 0.1, DurationTransformNone)

	// n=10, median = (durations[4] + durations[5]) / 2 = (9ms + 10ms) / 2 = 9.5ms
	assert.Equal(t, (9*ms+10*ms)/2, median)
	assert.ElementsMatch(t, []int{8, 9}, outlierIndices)
	assert.Len(t, normalIndices, 8)
}

func TestDetectOutliersMAD_ZeroMAD(t *testing.T) {
	ms := time.Millisecond

	// All same value except one spike.
	durations := []time.Duration{
		10 * ms, 10 * ms, 10 * ms, 10 * ms, 10 * ms, 10 * ms,
		1000 * ms, // spike
	}

	outlierIndices, normalIndices, median := runDetector(t, durations, OutlierMethodMAD, 3.0, 0.1, DurationTransformNone)

	assert.Equal(t, 10*ms, median)
	// With MAD=0 and 10% min threshold, threshold = 10ms * 1.1 = 11ms
	// 1000ms > 11ms, so it's still an outlier
	assert.Equal(t, []int{6}, outlierIndices)
	assert.Len(t, normalIndices, 6)
}

func TestDetectOutliersMAD_BimodalDistribution(t *testing.T) {
	ms := time.Millisecond

	// Cache hit/miss pattern: bimodal distribution
	// Fast (cache hits): 5-15ms
	// Slow (cache misses): 100-120ms
	durations := []time.Duration{
		5 * ms, 7 * ms, 8 * ms, 10 * ms, 12 * ms, 15 * ms, // cache hits
		100 * ms, 110 * ms, 120 * ms, // cache misses
	}

	outlierIndices, normalIndices, median := runDetector(t, durations, OutlierMethodMAD, 3.0, 0.1, DurationTransformNone)

	// Median should be around 12ms
	assert.Equal(t, 12*ms, median)
	// The slow cache misses should be outliers
	assert.ElementsMatch(t, []int{6, 7, 8}, outlierIndices)
	assert.Len(t, normalIndices, 6)
}

func TestDetectOutliersMAD_SmallGroup(t *testing.T) {
	ms := time.Millisecond

	// 7 spans (minimum valid group size)
	durations := []time.Duration{
		5 * ms, 6 * ms, 7 * ms, 8 * ms, 9 * ms, 10 * ms,
		500 * ms, // outlier
	}

	outlierIndices, normalIndices, median := runDetector(t, durations, OutlierMethodMAD, 3.0, 0.1, DurationTransformNone)

	assert.Equal(t, 8*ms, median)
	assert.Equal(t, []int{6}, outlierIndices)
	assert.Len(t, normalIndices, 6)
}

func TestAnalyzeOutliers_MethodSelection(t *testing.T) {
	ms := time.Millisecond

	durations := []time.Duration{
		5 * ms, 6 * ms, 7 * ms, 8 * ms, 9 * ms, 10 * ms, 11 * ms, 12 * ms,
		500 * ms, 600 * ms, // outliers
	}
	attrs := []map[string]string{
		{"key": "a"},
		{"key": "b"},
		{"key": "c"},
		{"key": "d"},
		{"key": "e"},
		{"key": "f"},
		{"key": "g"},
		{"key": "h"},
		{"key": "i"},
		{"key": "j"},
	}

	tests := []struct {
		name   string
		method OutlierMethod
	}{
		{
			name:   "default (empty) uses IQR",
			method: "",
		},
		{
			name:   "explicit IQR",
			method: OutlierMethodIQR,
		},
		{
			name:   "explicit MAD",
			method: OutlierMethodMAD,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := makeNodesWithAttrs(durations, attrs)
			cfg := OutlierAnalysisConfig{
				Method:                         tt.method,
				IQRMultiplier:                  1.5,
				MADMultiplier:                  3.0,
				MinGroupSize:                   7,
				CorrelationMinOccurrence:       0.5,
				CorrelationMaxNormalOccurrence: 0.5,
				MaxCorrelatedAttributes:        5,
			}

			result := analyzeOutliers(nodes, cfg)

			require.NotNil(t, result)
			assert.True(t, result.hasOutliers)
		})
	}
}

func TestMADvsIQR_Comparison(t *testing.T) {
	ms := time.Millisecond

	// Distribution with moderate outliers
	// MAD should be more sensitive to this pattern
	durations := []time.Duration{
		10 * ms, 11 * ms, 12 * ms, 13 * ms, 14 * ms,
		15 * ms, 16 * ms, 17 * ms, 18 * ms, 19 * ms,
		100 * ms, // moderate outlier
	}

	nodes := makeNodesWithAttrs(durations, nil)

	iqrCfg := OutlierAnalysisConfig{
		Method:                         OutlierMethodIQR,
		IQRMultiplier:                  1.5,
		MADMultiplier:                  3.0,
		MinGroupSize:                   7,
		CorrelationMinOccurrence:       0.5,
		CorrelationMaxNormalOccurrence: 0.5,
		MaxCorrelatedAttributes:        5,
	}

	madCfg := OutlierAnalysisConfig{
		Method:                         OutlierMethodMAD,
		IQRMultiplier:                  1.5,
		MADMultiplier:                  3.0,
		MinGroupSize:                   7,
		CorrelationMinOccurrence:       0.5,
		CorrelationMaxNormalOccurrence: 0.5,
		MaxCorrelatedAttributes:        5,
	}

	iqrResult := analyzeOutliers(nodes, iqrCfg)
	madResult := analyzeOutliers(nodes, madCfg)

	require.NotNil(t, iqrResult)
	require.NotNil(t, madResult)

	// Both should detect the 100ms outlier
	assert.True(t, iqrResult.hasOutliers)
	assert.True(t, madResult.hasOutliers)

	// Both should have the same median
	assert.Equal(t, iqrResult.median, madResult.median)
}

func TestMinOutlierThresholdPercent(t *testing.T) {
	ms := time.Millisecond

	// All same value except one that's slightly above (5% above median)
	// With 10% threshold, it should NOT be an outlier
	// With 0% threshold, it SHOULD be an outlier
	durations := []time.Duration{
		100 * ms, 100 * ms, 100 * ms, 100 * ms, 100 * ms, 100 * ms,
		105 * ms, // 5% above median
	}

	for _, tt := range []struct {
		name                string
		method              OutlierMethod
		multiplier          float64
		minThresholdPercent float64
		wantOutliers        []int
	}{
		// The statistical spread is zero either way, so the minimum threshold
		// is what decides: 100ms * 1.10 = 110ms excludes the 105ms span, while
		// 100ms * 1.00 = 100ms admits it.
		{"IQR with 10% threshold excludes 5% deviation", OutlierMethodIQR, 1.5, 0.10, nil},
		{"IQR with 0% threshold includes 5% deviation", OutlierMethodIQR, 1.5, 0.0, []int{6}},
		{"MAD with 10% threshold excludes 5% deviation", OutlierMethodMAD, 3.0, 0.10, nil},
		{"MAD with 0% threshold includes 5% deviation", OutlierMethodMAD, 3.0, 0.0, []int{6}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			outlierIndices, normalIndices, median := runDetector(t, durations, tt.method, tt.multiplier, tt.minThresholdPercent, DurationTransformNone)

			assert.Equal(t, 100*ms, median)
			assert.ElementsMatch(t, tt.wantOutliers, outlierIndices)
			assert.Len(t, normalIndices, 7-len(tt.wantOutliers))
		})
	}
}

func TestLogTransform_IQRThreshold(t *testing.T) {
	ms := time.Millisecond

	// q1 = 8ms and q3 = 32ms. In log space the threshold
	// ln(q3) + 1.5*(ln(q3)-ln(q1)) inverts to q3*(q3/q1)^1.5, which is
	// 32ms * 4^1.5 = 256ms exactly. The 255ms and 257ms spans straddle it,
	// which pins the threshold rather than only the direction of the change.
	durations := []time.Duration{
		6 * ms, 7 * ms, 8 * ms, 10 * ms, 12 * ms, 16 * ms, 20 * ms, 24 * ms,
		32 * ms, 255 * ms, 257 * ms,
	}

	outlierIndices, normalIndices, median := runDetector(t, durations, OutlierMethodIQR, 1.5, 0.1, DurationTransformLog)

	assert.Equal(t, 16*ms, median)
	assert.Equal(t, []int{10}, outlierIndices)
	assert.Len(t, normalIndices, 10)

	// Untransformed, the threshold is q3 + 1.5*IQR = 68ms, which also flags the
	// 255ms span.
	noneOutliers, _, _ := runDetector(t, durations, OutlierMethodIQR, 1.5, 0.1, DurationTransformNone)
	assert.ElementsMatch(t, []int{9, 10}, noneOutliers)
}

func TestLogTransform_MADThreshold(t *testing.T) {
	ms := time.Millisecond

	// The median is 100ms and the median absolute deviation in log space is
	// ln(2), so the threshold inverts to 100ms * 2^(3*1.4826) = 2182.33ms. The
	// two largest spans straddle it, and their own deviations are too extreme to
	// move the median deviation that sets the threshold.
	durations := []time.Duration{
		25 * ms, 50 * ms, 50 * ms, 100 * ms, 100 * ms, 100 * ms, 100 * ms,
		200 * ms, 200 * ms, 2182 * ms, 2183 * ms,
	}

	outlierIndices, normalIndices, median := runDetector(t, durations, OutlierMethodMAD, 3.0, 0.1, DurationTransformLog)

	assert.Equal(t, 100*ms, median)
	assert.Equal(t, []int{10}, outlierIndices)
	assert.Len(t, normalIndices, 10)
}

func TestLogTransform_FlagsFewerOnHeavyTail(t *testing.T) {
	ms := time.Millisecond

	// A dense bulk plus a smooth multiplicative tail, which is the shape that
	// makes an untransformed threshold land only a few percentiles above q3.
	durations := []time.Duration{
		8 * ms, 9 * ms, 9 * ms, 10 * ms, 10 * ms, 10 * ms, 11 * ms, 11 * ms,
		11 * ms, 12 * ms, 12 * ms, 12 * ms, 13 * ms, 13 * ms, 14 * ms, 14 * ms,
		15 * ms, 16 * ms, 17 * ms, 18 * ms, 19 * ms, 20 * ms, 21 * ms, 22 * ms,
		24 * ms, 26 * ms, 28 * ms, 31 * ms, 34 * ms, 38 * ms,
		45 * ms, 55 * ms, 70 * ms, 95 * ms, 130 * ms, 185 * ms, 260 * ms,
		370 * ms, 520 * ms, 740 * ms,
	}

	for _, tt := range []struct {
		name         string
		method       OutlierMethod
		multiplier   float64
		noneOutliers int
		logOutliers  int
	}{
		{"iqr", OutlierMethodIQR, 1.5, 7, 3},
		{"mad", OutlierMethodMAD, 3.0, 9, 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			none, _, _ := runDetector(t, durations, tt.method, tt.multiplier, 0.1, DurationTransformNone)
			logged, _, _ := runDetector(t, durations, tt.method, tt.multiplier, 0.1, DurationTransformLog)

			assert.Len(t, none, tt.noneOutliers)
			assert.Len(t, logged, tt.logOutliers)
			// Every span the log transform flags is also flagged without it: the
			// cut moves outward, it does not move sideways.
			assert.Subset(t, none, logged)
		})
	}
}

func TestLogTransform_ZeroDurations(t *testing.T) {
	ms := time.Millisecond

	t.Run("all zero durations flag nothing", func(t *testing.T) {
		durations := make([]time.Duration, 7)

		for _, method := range []OutlierMethod{OutlierMethodIQR, OutlierMethodMAD} {
			t.Run(string(method), func(t *testing.T) {
				outlierIndices, normalIndices, median := runDetector(t, durations, method, 3.0, 0.1, DurationTransformLog)

				assert.Equal(t, time.Duration(0), median)
				assert.Empty(t, outlierIndices)
				assert.Len(t, normalIndices, 7)
			})
		}
	})

	t.Run("instantaneous spans do not flag the rest of the group", func(t *testing.T) {
		// A quarter of the group is instantaneous, so q1 floors at 1ns and the
		// resulting spread is wide enough to put the threshold beyond every
		// span, including one at 5s. Instantaneous spans widen the spread rather
		// than breaking the transform.
		durations := []time.Duration{
			0, 0, 10 * ms, 20 * ms, 30 * ms, 40 * ms, 5000 * ms,
		}

		outlierIndices, normalIndices, median := runDetector(t, durations, OutlierMethodIQR, 1.5, 0.1, DurationTransformLog)

		assert.Equal(t, 20*ms, median)
		assert.Empty(t, outlierIndices)
		assert.Len(t, normalIndices, 7)
	})
}

func TestAnalyzeOutliers_DurationTransformMatrix(t *testing.T) {
	ms := time.Millisecond

	durations := []time.Duration{
		10 * ms, 11 * ms, 12 * ms, 13 * ms, 14 * ms, 15 * ms, 16 * ms, 17 * ms,
		18 * ms, 19 * ms, 5000 * ms,
	}

	for _, tt := range []struct {
		name      string
		method    OutlierMethod
		transform DurationTransform
	}{
		{"iqr none", OutlierMethodIQR, DurationTransformNone},
		{"iqr log", OutlierMethodIQR, DurationTransformLog},
		{"mad none", OutlierMethodMAD, DurationTransformNone},
		{"mad log", OutlierMethodMAD, DurationTransformLog},
		{"empty transform defaults to none", OutlierMethodIQR, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			nodes := makeNodesWithAttrs(durations, nil)
			cfg := OutlierAnalysisConfig{
				Method:                         tt.method,
				DurationTransform:              tt.transform,
				IQRMultiplier:                  1.5,
				MADMultiplier:                  3.0,
				MinGroupSize:                   7,
				CorrelationMinOccurrence:       0.5,
				CorrelationMaxNormalOccurrence: 0.5,
				MaxCorrelatedAttributes:        5,
			}

			result := analyzeOutliers(nodes, cfg)

			// The 5000ms span is over three hundred times the median, so every
			// combination has to flag it and only it.
			require.NotNil(t, result)
			assert.True(t, result.hasOutliers)
			assert.Equal(t, []int{10}, result.outlierIndices)
		})
	}
}

func TestAnalyzeOutliers_UnfinishedSpanIsNotAnOutlier(t *testing.T) {
	ms := time.Millisecond
	baseTime := pcommon.NewTimestampFromTime(time.Now())

	newSpan := func(end pcommon.Timestamp) *spanNode {
		span := ptrace.NewSpan()
		span.SetName("test")
		span.SetStartTimestamp(baseTime)
		span.SetEndTimestamp(end)
		return &spanNode{span: span}
	}

	nodes := make([]*spanNode, 0, 7)
	for range 6 {
		nodes = append(nodes, newSpan(pcommon.NewTimestampFromTime(baseTime.AsTime().Add(10*ms))))
	}
	// An unfinished span still carries end_time_unix_nano == 0, so end - start
	// underflows the unsigned timestamp type. Reading that as an enormous
	// positive duration would sort it last and flag it every time.
	nodes = append(nodes, newSpan(0))

	cfg := OutlierAnalysisConfig{
		IQRMultiplier:                  1.5,
		MADMultiplier:                  3.0,
		MinGroupSize:                   7,
		CorrelationMinOccurrence:       0.5,
		CorrelationMaxNormalOccurrence: 0.5,
		MaxCorrelatedAttributes:        5,
		MinOutlierThresholdPercent:     0.1,
	}

	for _, transform := range []DurationTransform{DurationTransformNone, DurationTransformLog} {
		t.Run(string(transform), func(t *testing.T) {
			cfg.DurationTransform = transform
			result := analyzeOutliers(nodes, cfg)

			require.NotNil(t, result)
			assert.NotContains(t, result.outlierIndices, 6)
			assert.Contains(t, result.normalIndices, 6)
		})
	}
}
