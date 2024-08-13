// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/DataDog/agent-payload/v5/gogen"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestHandleSketchPayload(t *testing.T) {
	tests := []struct {
		name                      string
		sketchPayload             gogen.SketchPayload
		expectedSketchesCount     int
		expectedDogsketchesCounts []int
	}{
		{
			name: "Test simple sketch payload with single sketch",
			sketchPayload: gogen.SketchPayload{
				Sketches: []gogen.SketchPayload_Sketch{
					{
						Metric:        "Test1",
						Host:          "Host1",
						Tags:          []string{"env:tag1", "version:tag2"},
						Distributions: []gogen.SketchPayload_Sketch_Distribution{},
						Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
							{
								Ts:  400,
								Cnt: 6,
								Min: 1,
								Max: 3,
								Avg: 2.3333,
								Sum: 14.0,
								K:   []int32{1338, 1383, 1409},
								N:   []uint32{1, 2, 3},
							},
						},
					},
				},
			},
			expectedSketchesCount:     1,
			expectedDogsketchesCounts: []int{1},
		},
		{
			name: "Test simple sketch payload with multiple dogsketches",
			sketchPayload: gogen.SketchPayload{
				Sketches: []gogen.SketchPayload_Sketch{
					{
						Metric:        "Test1",
						Host:          "Host1",
						Tags:          []string{"env:tag1", "version:tag2"},
						Distributions: []gogen.SketchPayload_Sketch_Distribution{},
						Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
							{
								Ts:  400,
								Cnt: 6,
								Min: 1,
								Max: 3,
								Avg: 2.3333,
								Sum: 14.0,
								K:   []int32{1338, 1383, 1409},
								N:   []uint32{1, 2, 3},
							},
							{
								Ts:  500,
								Cnt: 15,
								Min: 4,
								Max: 5,
								Avg: 4.7333,
								Sum: 71.0,
								K:   []int32{1427, 1442, 1454},
								N:   []uint32{4, 5, 6},
							},
						},
					},
				},
			},
			expectedSketchesCount:     1,
			expectedDogsketchesCounts: []int{2},
		},
		{
			name: "Test sketch payload with multiple sketches",
			sketchPayload: gogen.SketchPayload{
				Sketches: []gogen.SketchPayload_Sketch{
					{
						Metric:        "Test1",
						Host:          "Host1",
						Tags:          []string{"env:tag1", "version:tag2"},
						Distributions: []gogen.SketchPayload_Sketch_Distribution{},
						Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
							{
								Ts:  400,
								Cnt: 6,
								Min: 1,
								Max: 3,
								Avg: 2.3333,
								Sum: 14.0,
								K:   []int32{1338, 1383, 1409},
								N:   []uint32{1, 2, 3},
							},
						},
					},
					{
						Metric:        "Test2",
						Host:          "Host1",
						Tags:          []string{"env:tag1", "version:tag2"},
						Distributions: []gogen.SketchPayload_Sketch_Distribution{},
						Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
							{
								Ts:  400,
								Cnt: 6,
								Min: 1,
								Max: 3,
								Avg: 2.3333,
								Sum: 14.0,
								K:   []int32{1338, 1383, 1409},
								N:   []uint32{1, 2, 3},
							},
						},
					},
				},
			},
			expectedSketchesCount:     2,
			expectedDogsketchesCounts: []int{1, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb, err := tt.sketchPayload.Marshal()
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPost, "/api/beta/sketches", io.NopCloser(bytes.NewReader(pb)))
			require.Nil(t, err)

			mt := createMetricsTranslator()
			metrics, err := mt.HandleSketchesPayload(req)
			require.Nil(t, err)
			require.Equal(t, tt.expectedSketchesCount, len(metrics))
			for i, metric := range metrics {
				require.Equal(t, tt.expectedDogsketchesCounts[i], len(metric.Dogsketches))
			}
		})
	}
}

func TestTranslateSketches(t *testing.T) {
	tests := []struct {
		name     string
		sketches []gogen.SketchPayload_Sketch
	}{
		{
			name: "Single sketch with only positive buckets and no zero bucket",
			sketches: []gogen.SketchPayload_Sketch{
				{
					Metric:        "Test1",
					Host:          "Host1",
					Tags:          []string{"env:tag1", "version:tag2"},
					Distributions: []gogen.SketchPayload_Sketch_Distribution{},
					Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
						{
							Ts:  400,
							Cnt: 1029,
							Min: 1.0,
							Max: 6.0,
							Avg: 3.0,
							Sum: 2038.0,
							K:   []int32{0, 1338, 1345, 1383, 1409, 1427, 1442, 1454, 1464},
							N:   []uint32{13, 152, 75, 231, 97, 55, 101, 239, 66},
						},
					},
				},
			},
		},
		{
			name: "Single sketch with only negative buckets and no zero bucket",
			sketches: []gogen.SketchPayload_Sketch{
				{
					Metric:        "Test1",
					Host:          "Host1",
					Tags:          []string{"env:tag1", "version:tag2"},
					Distributions: []gogen.SketchPayload_Sketch_Distribution{},
					Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
						{
							Ts:  400,
							Cnt: 941,
							Min: -6.0,
							Max: -1.0,
							Avg: -3.0,
							Sum: 2038.0,
							K:   []int32{-1464, -1454, -1442, -1427, -1409, -1383, -1338},
							N:   []uint32{152, 231, 97, 55, 101, 239, 66},
						},
					},
				},
			},
		},
		{
			name: "Single sketch with negative and positive buckets and no zero bucket",
			sketches: []gogen.SketchPayload_Sketch{
				{
					Metric:        "Test1",
					Host:          "Host1",
					Tags:          []string{"env:tag1", "version:tag2"},
					Distributions: []gogen.SketchPayload_Sketch_Distribution{},
					Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
						{
							Ts:  400,
							Cnt: 1952,
							Min: 1.0,
							Max: 6.0,
							Avg: 3.0,
							Sum: 1019.0,
							K:   []int32{-1464, -1454, -1442, -1427, -1409, -1383, -1338, 1338, 1383, 1409, 1427, 1442, 1454, 1464},
							N:   []uint32{152, 231, 97, 55, 101, 239, 66, 43, 99, 123, 62, 194, 251, 239},
						},
					},
				},
			},
		},
		{
			name: "Single sketch with only positive buckets and zero bucket",
			sketches: []gogen.SketchPayload_Sketch{
				{
					Metric:        "Test1",
					Host:          "Host1",
					Tags:          []string{"env:tag1", "version:tag2"},
					Distributions: []gogen.SketchPayload_Sketch_Distribution{},
					Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
						{
							Ts:  400,
							Cnt: 954,
							Min: 1.0,
							Max: 6.0,
							Avg: 3.0,
							Sum: 2049.0,
							K:   []int32{0, 1338, 1383, 1409, 1427, 1442, 1454, 1464},
							N:   []uint32{13, 152, 231, 97, 55, 101, 239, 66},
						},
					},
				},
			},
		},
		{
			name: "Single sketch with only negative buckets and no zero bucket",
			sketches: []gogen.SketchPayload_Sketch{
				{
					Metric:        "Test1",
					Host:          "Host1",
					Tags:          []string{"env:tag1", "version:tag2"},
					Distributions: []gogen.SketchPayload_Sketch_Distribution{},
					Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
						{
							Ts:  400,
							Cnt: 941,
							Min: -6.0,
							Max: -1.0,
							Avg: -3.0,
							Sum: -2049,
							K:   []int32{-1464, -1454, -1442, -1427, -1409, -1383, -1338},
							N:   []uint32{152, 231, 97, 55, 101, 239, 66},
						},
					},
				},
			},
		},
		{
			name: "Single sketch with negative and positive buckets and zero bucket",
			sketches: []gogen.SketchPayload_Sketch{
				{
					Metric:        "Test1",
					Host:          "Host1",
					Tags:          []string{"env:tag1", "version:tag2"},
					Distributions: []gogen.SketchPayload_Sketch_Distribution{},
					Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
						{
							Ts:  400,
							Cnt: 1964,
							Min: 1.0,
							Max: 6.0,
							Avg: 3.0,
							Sum: 1589.0,
							K:   []int32{-1464, -1454, -1442, -1427, -1409, -1383, -1338, 0, 1338, 1383, 1409, 1427, 1442, 1454, 1464},
							N:   []uint32{152, 231, 97, 55, 101, 239, 66, 12, 43, 99, 123, 62, 194, 251, 239},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := createMetricsTranslator()
			result := mt.TranslateSketches(tt.sketches)
			require.Equal(t, 1, result.MetricCount())
			require.Equal(t, 1, result.DataPointCount())
			metrics := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			require.Equal(t, 1, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())

			metric := metrics.At(0)
			require.Equal(t, pmetric.MetricTypeExponentialHistogram, metric.Type())

			for _, sketch := range tt.sketches {
				require.Equal(t, sketch.GetMetric(), metric.Name())
				for i, dogsketch := range sketch.Dogsketches {
					m := metric.ExponentialHistogram().DataPoints().At(i)
					require.Equal(t, pcommon.Timestamp(dogsketch.Ts*time.Second.Nanoseconds()), m.Timestamp())
					require.Equal(t, uint64(dogsketch.Cnt), m.Count())
					require.Equal(t, dogsketch.Sum, m.Sum())
					require.Equal(t, dogsketch.Min, m.Min())
					require.Equal(t, dogsketch.Max, m.Max())
					require.Equal(t, m.Count(), totalHistBucketCounts(m)) // Ensure that buckets contain same number of counts as total count
				}
			}
		})
	}
}

func TestSketchTemporality(t *testing.T) {
	tests := []struct {
		name     string
		sketches []gogen.SketchPayload_Sketch
	}{
		{
			name: "Two metrics with multiple data points",
			sketches: []gogen.SketchPayload_Sketch{
				{
					Metric:        "Test1",
					Host:          "Host1",
					Tags:          []string{"version:tag1"},
					Distributions: []gogen.SketchPayload_Sketch_Distribution{},
					Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
						{
							Ts:  100,
							Cnt: 1029,
							Min: 1.0,
							Max: 6.0,
							Avg: 3.0,
							Sum: 2038.0,
							K:   []int32{0, 1338, 1345, 1383, 1409, 1427, 1442, 1454, 1464},
							N:   []uint32{13, 152, 75, 231, 97, 55, 101, 239, 66},
						},
						{
							Ts:  200,
							Cnt: 1029,
							Min: 1.0,
							Max: 6.0,
							Avg: 3.0,
							Sum: 2038.0,
							K:   []int32{0, 1338, 1345, 1383, 1409, 1427, 1442, 1454, 1464},
							N:   []uint32{13, 152, 75, 231, 97, 55, 101, 239, 66},
						},
						{
							Ts:  300,
							Cnt: 1029,
							Min: 1.0,
							Max: 6.0,
							Avg: 3.0,
							Sum: 2038.0,
							K:   []int32{0, 1338, 1345, 1383, 1409, 1427, 1442, 1454, 1464},
							N:   []uint32{13, 152, 75, 231, 97, 55, 101, 239, 66},
						},
					},
				},
				{
					Metric:        "Test2",
					Host:          "Host2",
					Tags:          []string{"env:tag1", "version:tag2"},
					Distributions: []gogen.SketchPayload_Sketch_Distribution{},
					Dogsketches: []gogen.SketchPayload_Sketch_Dogsketch{
						{
							Ts:  20,
							Cnt: 1029,
							Min: 1.0,
							Max: 6.0,
							Avg: 3.0,
							Sum: 2038.0,
							K:   []int32{0, 1338, 1345, 1383, 1409, 1427, 1442, 1454, 1464},
							N:   []uint32{13, 152, 75, 231, 97, 55, 101, 239, 66},
						},
						{
							Ts:  30,
							Cnt: 1029,
							Min: 1.0,
							Max: 6.0,
							Avg: 3.0,
							Sum: 2038.0,
							K:   []int32{0, 1338, 1345, 1383, 1409, 1427, 1442, 1454, 1464},
							N:   []uint32{13, 152, 75, 231, 97, 55, 101, 239, 66},
						},
						{
							Ts:  40,
							Cnt: 1029,
							Min: 1.0,
							Max: 6.0,
							Avg: 3.0,
							Sum: 2038.0,
							K:   []int32{0, 1338, 1345, 1383, 1409, 1427, 1442, 1454, 1464},
							N:   []uint32{13, 152, 75, 231, 97, 55, 101, 239, 66},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := createMetricsTranslator()
			result := mt.TranslateSketches(tt.sketches)
			require.Equal(t, 2, result.ResourceMetrics().Len())
			require.Equal(t, 2, result.MetricCount())
			require.Equal(t, 6, result.DataPointCount())

			requireScope(t, result, pcommon.NewMap(), component.NewDefaultBuildInfo().Version)

			metric1 := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			require.Equal(t, 1, result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
			require.Equal(t, pmetric.MetricTypeExponentialHistogram, metric1.At(0).Type())

			metric2 := result.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics()
			require.Equal(t, 1, result.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().Len())
			require.Equal(t, pmetric.MetricTypeExponentialHistogram, metric2.At(0).Type())

			var lastTimestamp pcommon.Timestamp
			for i := 0; i < metric1.At(0).ExponentialHistogram().DataPoints().Len(); i++ {
				m := metric1.At(0).ExponentialHistogram().DataPoints().At(i)
				if i == 0 {
					require.Equal(t, m.StartTimestamp(), pcommon.Timestamp(0))
				} else {
					require.Equal(t, m.StartTimestamp(), lastTimestamp)
				}
				lastTimestamp = m.Timestamp()
			}
			for i := 0; i < metric2.At(0).ExponentialHistogram().DataPoints().Len(); i++ {
				m := metric2.At(0).ExponentialHistogram().DataPoints().At(i)
				if i == 0 {
					require.Equal(t, m.StartTimestamp(), pcommon.Timestamp(0))
				} else {
					require.Equal(t, m.StartTimestamp(), lastTimestamp)
				}
				lastTimestamp = m.Timestamp()
			}
		})
	}
}
