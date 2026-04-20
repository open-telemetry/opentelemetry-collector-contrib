// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateAggregationData_CumulativeHistogramBuckets(t *testing.T) {
	p := &spanPruningProcessor{
		config: &Config{
			AggregationHistogramBuckets: []time.Duration{
				10 * time.Millisecond,
				50 * time.Millisecond,
				100 * time.Millisecond,
			},
		},
	}

	nodes := createSpanNodesWithDurations(t, []int64{
		int64(5 * time.Millisecond),
		int64(15 * time.Millisecond),
		int64(25 * time.Millisecond),
		int64(75 * time.Millisecond),
		int64(150 * time.Millisecond),
	})

	data := p.calculateAggregationData(nodes)
	assert.Equal(t, []int64{1, 3, 4, 5}, data.bucketCounts)
}

func TestCalculateAggregationData_NoHistogramBuckets(t *testing.T) {
	p := &spanPruningProcessor{
		config: &Config{},
	}

	nodes := createSpanNodesWithDurations(t, []int64{
		int64(5 * time.Millisecond),
		int64(15 * time.Millisecond),
	})

	data := p.calculateAggregationData(nodes)
	assert.Nil(t, data.bucketCounts)
}
