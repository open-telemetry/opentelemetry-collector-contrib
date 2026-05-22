// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudmonitoringreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestIngestDelay(t *testing.T) {
	fallback := 3 * time.Minute

	testCases := map[string]struct {
		desc *metric.MetricDescriptor
		want time.Duration
	}{
		"metadata populated uses descriptor value": {
			desc: &metric.MetricDescriptor{
				Metadata: &metric.MetricDescriptor_MetricDescriptorMetadata{
					IngestDelay: durationpb.New(210 * time.Second),
				},
			},
			want: 210 * time.Second,
		},
		"metadata nil falls back to default": {
			desc: &metric.MetricDescriptor{},
			want: fallback,
		},
		"metadata present but ingest delay zero falls back to default": {
			desc: &metric.MetricDescriptor{
				Metadata: &metric.MetricDescriptor_MetricDescriptorMetadata{
					IngestDelay: durationpb.New(0),
				},
			},
			want: fallback,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, ingestDelay(tc.desc, fallback))
		})
	}
}

func TestCalculateStartEndTime(t *testing.T) {
	interval := 1 * time.Minute
	delay := 2 * time.Minute

	startTime, endTime := calculateStartEndTime(interval, delay)

	gap := endTime.Sub(startTime)
	assert.Equal(t, interval, gap, "window length must equal interval")

	timeSinceEnd := time.Since(endTime)
	assert.GreaterOrEqual(t, timeSinceEnd, delay-time.Second, "endTime should be at least 'delay' ago")
	assert.LessOrEqual(t, timeSinceEnd, delay+time.Second, "endTime should be at most 'delay' ago plus jitter")
}

func TestCalculateStartEndTimeIntervalCap(t *testing.T) {
	interval := 48 * time.Hour
	delay := 1 * time.Minute

	startTime, endTime := calculateStartEndTime(interval, delay)

	gap := endTime.Sub(startTime)
	assert.Equal(t, 23*time.Hour, gap, "interval must be capped at 23h")
}
