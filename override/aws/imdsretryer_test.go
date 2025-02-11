// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aws // import "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"

import (
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/stretchr/testify/assert"
)

func Test_IMDSRetryer_ShouldRetry(t *testing.T) {
	tests := []struct {
		name string
		req  *request.Request
		want bool
	}{
		{
			name: "ErrorIsNilDoNotRetry",
			req: &request.Request{
				Error: nil,
			},
			want: false,
		},
		{
			// no enum for status codes in request.Request nor http.Response
			name: "ErrorIsDefaultRetryable",
			req: &request.Request{
				Error: awserr.New("throttle me for 503", "throttle me for 503", nil),
				HTTPResponse: &http.Response{
					StatusCode: http.StatusServiceUnavailable,
				},
			},
			want: true,
		},
		{
			name: "ErrorIsEC2MetadataErrorRetryable",
			req: &request.Request{
				Error: awserr.New("EC2MetadataError", "EC2MetadataError", nil),
			},
			want: true,
		},
		{
			name: "ErrorIsAWSOtherErrorNotRetryable",
			req: &request.Request{
				Error: awserr.New("other", "other", nil),
			},
			want: false,
		},
		{
			// errors.New as a parent error will always retry due to fallback
			name: "ErrorIsAWSOtherWithParentErrorRetryable",
			req: &request.Request{
				Error: awserr.New("other", "other", errors.New("other")),
			},
			want: true,
		},
		{
			// errors.New will always retry due to fallback
			name: "ErrorIsOtherErrorRetryable",
			req: &request.Request{
				Error: errors.New("other"),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewIMDSRetryer(1).ShouldRetry(tt.req); got != tt.want {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNumberOfRetryTest(t *testing.T) {
	tests := []struct {
		name                  string
		expectedRetriesInput  int
		expectedRetriesOutput int
	}{
		{
			name:                  "expect 0 for 0",
			expectedRetriesInput:  0,
			expectedRetriesOutput: 0,
		},
		{
			name:                  "expect 5 for 5",
			expectedRetriesInput:  5,
			expectedRetriesOutput: 5,
		},
	}
	for _, tt := range tests {
		func() {
			t.Run(tt.name, func(t *testing.T) {
				newIMDSRetryer := NewIMDSRetryer(tt.expectedRetriesInput)
				assert.Equal(t, tt.expectedRetriesOutput, newIMDSRetryer.MaxRetries())
			})
		}()
	}
}
