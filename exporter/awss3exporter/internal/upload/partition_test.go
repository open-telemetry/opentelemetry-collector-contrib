// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configcompression"
)

func TestPartitionKeyInputsNewPartitionKey(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		inputs *PartitionKeyBuilder
		expect string
	}{
		{
			name: "empty values",
			inputs: &PartitionKeyBuilder{
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect: "/year=2024/month=01/day=24/hour=06/_fixed",
		},
		{
			name: "no compression set",
			inputs: &PartitionKeyBuilder{
				PartitionPrefix:     "/telemetry",
				PartitionTruncation: "minute",
				FilePrefix:          "signal-output-",
				Metadata:            "service-01_pod2",
				FileFormat:          "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect: "/telemetry/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics",
		},
		{
			name: "gzip compression set",
			inputs: &PartitionKeyBuilder{
				PartitionPrefix:     "/telemetry",
				PartitionTruncation: "minute",
				FilePrefix:          "signal-output-",
				Metadata:            "service-01_pod2",
				FileFormat:          "metrics",
				Compression:         configcompression.TypeGzip,
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect: "/telemetry/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics.gz",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := time.Date(2024, 0o1, 24, 6, 40, 20, 0, time.Local)

			assert.Equal(t, tc.expect, tc.inputs.Build(ts), "Must match the expected value")
		})
	}
}

func TestPartitionKeyInputsBucketPrefix(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		inputs *PartitionKeyBuilder
		expect string
	}{
		{
			name:   "no values provided",
			inputs: &PartitionKeyBuilder{},
			expect: "/year=2024/month=01/day=24/hour=06",
		},
		{
			name: "parition by minutes",
			inputs: &PartitionKeyBuilder{
				PartitionTruncation: "minute",
			},
			expect: "/year=2024/month=01/day=24/hour=06/minute=40",
		},
		{
			name: "unknown partition trunction value",
			inputs: &PartitionKeyBuilder{
				PartitionTruncation: "weekly",
			},
			expect: "/year=2024/month=01/day=24/hour=06",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := time.Date(2024, 0o1, 24, 6, 40, 20, 0, time.Local)

			assert.Equal(t, tc.expect, tc.inputs.bucketKeyPrefix(ts), "Must match the expected partition key")
		})
	}
}

func TestPartitionKeyInputsFilename(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		inputs *PartitionKeyBuilder
		expect string
	}{
		{
			name: "no values provided",
			inputs: &PartitionKeyBuilder{
				UniqueKeyFunc: func() string {
					return "buzz"
				},
			},
			expect: "_buzz",
		},
		{
			name: "no compression provided",
			inputs: &PartitionKeyBuilder{
				FilePrefix: "collector-capture-",
				FileFormat: "metrics",
				Metadata:   "service-01_pod1",
				UniqueKeyFunc: func() string {
					return "buzz"
				},
			},
			expect: "collector-capture-service-01_pod1_buzz.metrics",
		},
		{
			name: "valid compression set",
			inputs: &PartitionKeyBuilder{
				FilePrefix:  "collector-capture-",
				FileFormat:  "metrics",
				Metadata:    "service-01_pod1",
				Compression: configcompression.TypeGzip,
				UniqueKeyFunc: func() string {
					return "buzz"
				},
			},
			expect: "collector-capture-service-01_pod1_buzz.metrics.gz",
		},
		{
			name: "invalid compression set",
			inputs: &PartitionKeyBuilder{
				FilePrefix:  "collector-capture-",
				FileFormat:  "metrics",
				Metadata:    "service-01_pod1",
				Compression: configcompression.Type("foo"),
				UniqueKeyFunc: func() string {
					return "buzz"
				},
			},
			expect: "collector-capture-service-01_pod1_buzz.metrics",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expect, tc.inputs.fileName(), "Must match the expected value")
		})
	}
}

func TestPartitionKeyInputsUniqueKey(t *testing.T) {
	t.Parallel()

	// This test to is to help validate that a unique key
	// is not repeated

	seen := make(map[string]struct{})
	for i := 0; i < 500; i++ {
		uv := (&PartitionKeyBuilder{}).uniqueKey()
		_, ok := seen[uv]
		assert.False(t, ok, "Must not have repeated parition key %q", uv)
		seen[uv] = struct{}{}
	}
}
