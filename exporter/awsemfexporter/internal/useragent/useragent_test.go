// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package useragent

import (
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func TestUserAgent(t *testing.T) {
	testCases := map[string]struct {
		labelSets []map[string]string
		metrics   []string // Add metric names to test
		want      string
	}{
		"WithEmpty": {},
		"WithPartialAttributes": {
			labelSets: []map[string]string{
				{
					semconv.AttributeTelemetrySDKLanguage: "foo",
				},
				{
					semconv.AttributeTelemetryAutoVersion: "1.0",
				},
			},
		},
		"WithMultipleLanguages": {
			labelSets: []map[string]string{
				{
					semconv.AttributeTelemetrySDKLanguage: "foo",
					attributeTelemetryDistroVersion:       "1.1",
				},
				{
					semconv.AttributeTelemetrySDKLanguage: "bar",
					semconv.AttributeTelemetryAutoVersion: "2.0",
					attributeTelemetryDistroVersion:       "1.0",
				},
				{
					semconv.AttributeTelemetrySDKLanguage: "baz",
					semconv.AttributeTelemetryAutoVersion: "2.0",
				},
			},
			want: "telemetry-sdk (bar/1.0;baz/2.0;foo/1.1)",
		},
		"WithMultipleVersions": {
			labelSets: []map[string]string{
				{
					semconv.AttributeTelemetrySDKLanguage: "test",
					semconv.AttributeTelemetryAutoVersion: "1.1",
				},
				{
					semconv.AttributeTelemetrySDKLanguage: "test",
					attributeTelemetryDistroVersion:       "1.0",
				},
			},
			want: "telemetry-sdk (test/1.0)",
		},
		"WithTruncatedAttributes": {
			labelSets: []map[string]string{
				{
					semconv.AttributeTelemetrySDKLanguage: " incrediblyverboselanguagename",
					semconv.AttributeTelemetryAutoVersion: "notsemanticversioningversion",
				},
			},
			want: "telemetry-sdk (incrediblyverboselan/notsemanticversionin)",
		},
		"WithNvmeEBSMetrics": {
			metrics: []string{"node_diskio_ebs_total_read_ops"},
			want:    "feature:(nvme_ebs)",
		},
		"WithNvmeISMetrics": {
			metrics: []string{"node_diskio_instance_store_total_read_ops"},
			want:    "feature:(nvme_is)",
		},
		"WithBothNvmeMetrics": {
			metrics: []string{"node_diskio_ebs_total_read_ops", "node_diskio_instance_store_total_read_ops"},
			want:    "feature:(nvme_ebs nvme_is)",
		},
		"WithBothTelemetryAndEBS": {
			labelSets: []map[string]string{
				{
					semconv.AttributeTelemetrySDKLanguage: "test",
					attributeTelemetryDistroVersion:       "1.0",
				},
			},
			metrics: []string{"node_diskio_ebs_something"},
			want:    "telemetry-sdk (test/1.0) feature:(ci_ebs)",
		},
		"WithNonEBSMetrics": {
			metrics: []string{"some_other_metric"},
			want:    "",
		},
		"WithMultipleFeatures": {
			metrics: []string{"node_diskio_ebs_something", "node_diskio_ebs_something_else"},
			want:    "feature:(ci_ebs)",
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

			req := &request.Request{
				HTTPRequest: &http.Request{
					Header: http.Header{},
				},
			}
			userAgent.Handler().Fn(req)
			assert.Equal(t, testCase.want, req.HTTPRequest.Header.Get("User-Agent"))
		})
	}
}

func TestUserAgentExpiration(t *testing.T) {
	userAgent := newUserAgent(50 * time.Millisecond)
	req := &request.Request{
		HTTPRequest: &http.Request{
			Header: http.Header{},
		},
	}
	labels := map[string]string{
		semconv.AttributeTelemetrySDKLanguage: "test",
		semconv.AttributeTelemetryAutoVersion: "1.0",
	}
	userAgent.Process(labels)
	userAgent.handle(req)
	assert.Equal(t, "telemetry-sdk (test/1.0)", req.HTTPRequest.Header.Get("User-Agent"))

	// wait for expiration
	time.Sleep(100 * time.Millisecond)
	// reset user-agent header
	req.HTTPRequest.Header.Del("User-Agent")
	userAgent.handle(req)
	assert.Empty(t, req.HTTPRequest.Header.Get("User-Agent"))
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
