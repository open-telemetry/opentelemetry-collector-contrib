// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package appsignals

import (
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/stretchr/testify/assert"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
)

func TestUserAgent(t *testing.T) {
	testCases := map[string]struct {
		labelSets []map[string]string
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
