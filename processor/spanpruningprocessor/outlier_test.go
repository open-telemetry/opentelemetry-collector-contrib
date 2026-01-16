// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor"

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
				{"key": "a"}, {"key": "b"}, {"key": "c"}, {"key": "d"},
				{"key": "e"}, {"key": "f"}, {"key": "g"},
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
				{"a": "1"}, {"a": "2"}, {"a": "3"}, {"a": "4"},
				{"a": "5"}, {"a": "6"}, {"a": "7"},
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
				{"shard": "1"}, {"shard": "2"}, {"shard": "3"}, {"shard": "1"}, {"shard": "2"},
				{"shard": "1"}, {"shard": "2"}, // outliers have same distribution as normals
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
				{"key": "a"}, {"key": "b"}, {"key": "c"}, {"key": "d"}, {"key": "e"},
				{"key": "f"}, {"key": "g"}, {"key": "h"}, {"key": "i"}, {"key": "j"},
				{"key": "k"}, {"key": "l"},
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
				{"key": "a"}, {"key": "b"}, {"key": "c"}, {"key": "d"}, {"key": "e"},
				{"key": "f"}, {"key": "g"}, {"key": "h"}, {"key": "i"}, {"key": "j"},
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
				{"key": "a"}, {"key": "b"}, {"key": "c"}, {"key": "d"}, {"key": "e"},
				{"key": "f"}, {"key": "g"}, {"key": "h"}, {"key": "i"}, {"key": "j"},
				{"key": "k"}, {"key": "l"},
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
				{"shard": "1"}, {"shard": "2"}, {"shard": "3"}, {"shard": "1"},
				{"shard": "2"}, {"shard": "3"}, {"shard": "1"}, {"shard": "2"},
				{"shard": "3"}, {"shard": "1"},
				{"shard": "2"}, {"shard": "3"},
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
				{"cache": "hit"}, {"cache": "hit"}, {"cache": "hit"}, {"cache": "hit"}, {"cache": "hit"},
				{"cache": "hit"}, {"cache": "hit"}, {"cache": "hit"}, {"cache": "hit"}, {"cache": "hit"},
				{"cache": "miss"}, {"cache": "miss"}, // outliers
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
