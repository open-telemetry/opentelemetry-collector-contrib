// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package useragent

import (
	"context"
	"testing"
	"time"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// runUserAgent invokes ua.HandleBuild with a fresh smithy stack request and
// returns the resulting User-Agent header value.
func runUserAgent(t *testing.T, ua *UserAgent) string {
	t.Helper()
	req := smithyhttp.NewStackRequest().(*smithyhttp.Request)
	next := middleware.BuildHandlerFunc(func(_ context.Context, _ middleware.BuildInput) (middleware.BuildOutput, middleware.Metadata, error) {
		return middleware.BuildOutput{}, middleware.Metadata{}, nil
	})
	_, _, err := ua.HandleBuild(context.Background(), middleware.BuildInput{Request: req}, next)
	assert.NoError(t, err)
	return req.Header.Get("User-Agent")
}

func TestUserAgent(t *testing.T) {
	testCases := map[string]struct {
		labelSets []map[string]string
		metrics   []string
		want      string
	}{
		"WithEmpty": {},
		"WithPartialAttributes": {
			labelSets: []map[string]string{
				{
					string(semconv.TelemetrySDKLanguageKey): "foo",
				},
				{
					attributeTelemetryAutoVersion: "1.0",
				},
			},
		},
		"WithMultipleLanguages": {
			labelSets: []map[string]string{
				{
					string(semconv.TelemetrySDKLanguageKey):   "foo",
					string(semconv.TelemetryDistroVersionKey): "1.1",
				},
				{
					string(semconv.TelemetrySDKLanguageKey):   "bar",
					attributeTelemetryAutoVersion:             "2.0",
					string(semconv.TelemetryDistroVersionKey): "1.0",
				},
				{
					string(semconv.TelemetrySDKLanguageKey): "baz",
					attributeTelemetryAutoVersion:           "2.0",
				},
			},
			want: "telemetry-sdk (bar/1.0;baz/2.0;foo/1.1)",
		},
		"WithMultipleVersions": {
			labelSets: []map[string]string{
				{
					string(semconv.TelemetrySDKLanguageKey): "test",
					attributeTelemetryAutoVersion:           "1.1",
				},
				{
					string(semconv.TelemetrySDKLanguageKey):   "test",
					string(semconv.TelemetryDistroVersionKey): "1.0",
				},
			},
			want: "telemetry-sdk (test/1.0)",
		},
		"WithTruncatedAttributes": {
			labelSets: []map[string]string{
				{
					string(semconv.TelemetrySDKLanguageKey): " incrediblyverboselanguagename",
					attributeTelemetryAutoVersion:           "notsemanticversioningversion",
				},
			},
			want: "telemetry-sdk (incrediblyverboselan/notsemanticversionin)",
		},
		"WithEBSMetrics": {
			metrics: []string{"node_diskio_ebs_something"},
			want:    "feature:(ci_ebs)",
		},
		"WithLocalInstanceStoreMetrics": {
			metrics: []string{"node_diskio_instance_store_something"},
			want:    "feature:(ci_lis)",
		},
		"WithBothTelemetryAndEBS": {
			labelSets: []map[string]string{
				{
					string(semconv.TelemetrySDKLanguageKey):   "test",
					string(semconv.TelemetryDistroVersionKey): "1.0",
				},
			},
			metrics: []string{"node_diskio_ebs_something"},
			want:    "telemetry-sdk (test/1.0) feature:(ci_ebs)",
		},
		"WithBothTelemetryAndLocalInstanceStore": {
			labelSets: []map[string]string{
				{
					string(semconv.TelemetrySDKLanguageKey):   "test",
					string(semconv.TelemetryDistroVersionKey): "1.0",
				},
			},
			metrics: []string{"node_diskio_instance_store_something"},
			want:    "telemetry-sdk (test/1.0) feature:(ci_lis)",
		},
		"WithNonEBSMetrics": {
			metrics: []string{"some_other_metric"},
			want:    "",
		},
		"WithMultipleEBSMetrics": {
			metrics: []string{"node_diskio_ebs_something", "node_diskio_ebs_something_else"},
			want:    "feature:(ci_ebs)",
		},
		"WithMultipleLocalInstanceStoreMetrics": {
			metrics: []string{"node_diskio_instance_store_something", "node_diskio_instance_store_something_else"},
			want:    "feature:(ci_lis)",
		},
		"WithMixedEBSAndInstanceStoreMetrics": {
			metrics: []string{"node_diskio_ebs_something", "node_diskio_ebs_something", "node_diskio_instance_store_something", "node_diskio_instance_store_something"},
			want:    "feature:(ci_ebs ci_lis)",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			userAgent := NewUserAgent()
			for _, labelSet := range testCase.labelSets {
				userAgent.Process(labelSet)
			}

			if len(testCase.metrics) > 0 {
				metrics := createTestMetrics(testCase.metrics)
				userAgent.ProcessMetrics(metrics)
			}

			assert.Equal(t, testCase.want, runUserAgent(t, userAgent))
		})
	}
}

func TestUserAgentExpiration(t *testing.T) {
	userAgent := newUserAgent(50 * time.Millisecond)
	labels := map[string]string{
		string(semconv.TelemetrySDKLanguageKey): "test",
		attributeTelemetryAutoVersion:           "1.0",
	}
	userAgent.Process(labels)
	assert.Equal(t, "telemetry-sdk (test/1.0)", runUserAgent(t, userAgent))

	// wait for expiration
	time.Sleep(100 * time.Millisecond)
	assert.Empty(t, runUserAgent(t, userAgent))
}

func createTestMetrics(metricNames []string) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	ilm := rm.ScopeMetrics().AppendEmpty()

	for _, name := range metricNames {
		metric := ilm.Metrics().AppendEmpty()
		metric.SetName(name)
	}

	return metrics
}
