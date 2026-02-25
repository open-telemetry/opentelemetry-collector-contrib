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
		name           string
		inputs         *PartitionKeyBuilder
		expect         string
		overridePrefix string
	}{
		{
			name: "empty values",
			inputs: &PartitionKeyBuilder{
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "_fixed",
			overridePrefix: "",
		},
		{
			name: "fixed key with prefix",
			inputs: &PartitionKeyBuilder{
				PartitionPrefix: "telemetry",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "telemetry/_fixed",
			overridePrefix: "",
		},
		{
			name: "empty partition",
			inputs: &PartitionKeyBuilder{
				PartitionPrefix: "telemetry/foo",
				PartitionFormat: "",
				FilePrefix:      "signal-output-",
				FileFormat:      "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "telemetry/foo/signal-output-_fixed.metrics",
			overridePrefix: "",
		},
		{
			name: "no compression set",
			inputs: &PartitionKeyBuilder{
				PartitionPrefix: "/telemetry",
				PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:      "signal-output-",
				Metadata:        "service-01_pod2",
				FileFormat:      "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "/telemetry/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics",
			overridePrefix: "",
		},
		{
			name: "gzip compression set",
			inputs: &PartitionKeyBuilder{
				PartitionPrefix: "/telemetry",
				PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:      "signal-output-",
				Metadata:        "service-01_pod2",
				FileFormat:      "metrics",
				Compression:     configcompression.TypeGzip,
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "/telemetry/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics.gz",
			overridePrefix: "",
		},
		{
			name: "gzip compression set with overridePrefix",
			inputs: &PartitionKeyBuilder{
				PartitionPrefix: "/telemetry",
				PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:      "signal-output-",
				Metadata:        "service-01_pod2",
				FileFormat:      "metrics",
				Compression:     configcompression.TypeGzip,
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "/foo-prefix1/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics.gz",
			overridePrefix: "/foo-prefix1",
		},
		{
			name: "base path only",
			inputs: &PartitionKeyBuilder{
				PartitionBasePrefix: "base/path",
				PartitionFormat:     "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:          "signal-output-",
				Metadata:            "service-01_pod2",
				FileFormat:          "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "base/path/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics",
			overridePrefix: "",
		},
		{
			name: "base path with prefix",
			inputs: &PartitionKeyBuilder{
				PartitionBasePrefix: "base/path",
				PartitionPrefix:     "telemetry",
				PartitionFormat:     "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:          "signal-output-",
				Metadata:            "service-01_pod2",
				FileFormat:          "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "base/path/telemetry/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics",
			overridePrefix: "",
		},
		{
			name: "base path with prefix and override",
			inputs: &PartitionKeyBuilder{
				PartitionBasePrefix: "base/path",
				PartitionPrefix:     "telemetry",
				PartitionFormat:     "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:          "signal-output-",
				Metadata:            "service-01_pod2",
				FileFormat:          "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "base/path/override/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics",
			overridePrefix: "override",
		},
		{
			name: "base path with empty prefix",
			inputs: &PartitionKeyBuilder{
				PartitionBasePrefix: "base/path",
				PartitionPrefix:     "",
				PartitionFormat:     "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
				FilePrefix:          "signal-output-",
				Metadata:            "service-01_pod2",
				FileFormat:          "metrics",
				UniqueKeyFunc: func() string {
					return "fixed"
				},
			},
			expect:         "base/path/year=2024/month=01/day=24/hour=06/minute=40/signal-output-service-01_pod2_fixed.metrics",
			overridePrefix: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := time.Date(2024, 0o1, 24, 6, 40, 20, 0, time.Local)

			assert.Equal(t, tc.expect, tc.inputs.Build(ts, tc.overridePrefix), "Must match the expected value")
		})
	}
}

func TestPartitionKeyInputsBucketPrefix(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		inputs         *PartitionKeyBuilder
		expect         string
		overridePrefix string
	}{
		{
			name:           "no values provided",
			inputs:         &PartitionKeyBuilder{},
			expect:         "",
			overridePrefix: "",
		},
		{
			name: "partition by minutes",
			inputs: &PartitionKeyBuilder{
				PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
			},
			expect:         "year=2024/month=01/day=24/hour=06/minute=40",
			overridePrefix: "",
		},
		{
			name: "partition by hours",
			inputs: &PartitionKeyBuilder{
				PartitionFormat: "%Y/%m/%d/%H/%M",
			},
			expect:         "2024/01/24/06/40",
			overridePrefix: "",
		},
		{
			name:           "no values provided, overridePrefix is foo1",
			inputs:         &PartitionKeyBuilder{},
			expect:         "foo1/",
			overridePrefix: "foo1",
		},
		{
			name: "partition by minutes, overridePrefix is bar2",
			inputs: &PartitionKeyBuilder{
				PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
			},
			expect:         "bar2/year=2024/month=01/day=24/hour=06/minute=40",
			overridePrefix: "bar2",
		},
		{
			name: "partition by hours, overridePrefix is foo3",
			inputs: &PartitionKeyBuilder{
				PartitionFormat: "%Y/%m/%d/%H/%M",
			},
			expect:         "foo3/2024/01/24/06/40",
			overridePrefix: "foo3",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := time.Date(2024, 0o1, 24, 6, 40, 20, 0, time.Local)

			assert.Equal(t, tc.expect, tc.inputs.bucketKeyPrefix(ts, tc.overridePrefix), "Must match the expected partition key")
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
	for range 500 {
		uv := (&PartitionKeyBuilder{}).uniqueKey()
		_, ok := seen[uv]
		assert.False(t, ok, "Must not have repeated partition key %q", uv)
		seen[uv] = struct{}{}
	}

	// This test is to validate that the UUIDv7 unique key
	// is generated correctly, is unique, and is ordered by time.
	seen = make(map[string]struct{})
	lastKey := ""
	for range 500 {
		uv := (&PartitionKeyBuilder{UniqueKeyFunc: GenerateUUIDv7}).uniqueKey()
		_, ok := seen[uv]
		assert.False(t, ok, "Must not have repeated partition key %q", uv)
		seen[uv] = struct{}{}

		assert.Greater(t, uv, lastKey, "Must be greater than the last key %q", lastKey)
		lastKey = uv
	}

	for _, tc := range []struct {
		name   string
		inputs *PartitionKeyBuilder
		match  string
	}{
		{
			name: "default unique key",
			inputs: &PartitionKeyBuilder{
				FilePrefix: "collector-capture-",
				FileFormat: "metrics",
			},
			match: "collector-capture-_[0-9]+.metrics",
		},
		{
			name: "uuidv7 key",
			inputs: &PartitionKeyBuilder{
				FilePrefix:    "collector-capture-",
				FileFormat:    "metrics",
				UniqueKeyFunc: GenerateUUIDv7,
			},
			match: "collector-capture-_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}.metrics",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Regexp(t, tc.match, tc.inputs.fileName(), "Must match the expected regex pattern")
		})
	}
}
