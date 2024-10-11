// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter

import (
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/stretchr/testify/assert"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
)

func TestUserAgent(t *testing.T) {
	testCases := map[string]struct {
		labelSets []map[string]string
		want      []string
	}{
		"WithEmpty": {
			want: []string{""},
		},
		"WithPartialAttributes": {
			labelSets: []map[string]string{
				{
					semconv.AttributeTelemetrySDKLanguage: "foo",
				},
				{
					semconv.AttributeTelemetryDistroVersion: "1.0",
				},
			},
			want: []string{""},
		},
		"WithMultipleLanguages": {
			labelSets: []map[string]string{
				{
					semconv.AttributeTelemetrySDKLanguage: "foo",
					attributeTelemetryAutoVersion:       "1.1",
				},
				{
					semconv.AttributeTelemetrySDKLanguage: "bar",
					semconv.AttributeTelemetryDistroVersion: "2.0",
					attributeTelemetryAutoVersion:       "1.0",
				},
				{
					semconv.AttributeTelemetrySDKLanguage: "baz",
					semconv.AttributeTelemetryDistroVersion: "2.0",
				},
			},
			want: []string{"telemetry-sdk (bar/2.0;baz/2.0;foo/1.1)"},
		},
		"WithMultipleVersions": {
			labelSets: []map[string]string{
				{
					semconv.AttributeTelemetrySDKLanguage: "test",
					semconv.AttributeTelemetryDistroVersion: "1.1",
				},
				{
					semconv.AttributeTelemetrySDKLanguage: "test",
					attributeTelemetryAutoVersion:       "1.0",
				},
			},
			want: []string{
				"telemetry-sdk (test/1.1/1.0)",
				"telemetry-sdk (test/1.0/1.1)",
			},
		},
		"WithTruncatedAttributes": {
			labelSets: []map[string]string{
				{
					semconv.AttributeTelemetrySDKLanguage: " incrediblyverboselanguagename",
					semconv.AttributeTelemetryDistroVersion: "notsemanticversioningversion",
				},
			},
			want: []string{"telemetry-sdk (incrediblyverboselan/notsemanticversionin)"},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			userAgent := NewUserAgent()
			for _, labelSet := range testCase.labelSets {
				userAgent.Process(labelSet)
			}
			req := &request.Request{
				HTTPRequest: &http.Request{
					Header: http.Header{},
				},
			}
			userAgent.Handler().Fn(req)
			assert.Contains(t, testCase.want, req.HTTPRequest.Header.Get("User-Agent"))
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
		semconv.AttributeTelemetryDistroVersion: "1.0",
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
