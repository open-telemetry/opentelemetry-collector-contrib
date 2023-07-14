// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aws // import "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"

import (
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
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
					StatusCode: 503,
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
			r := IMDSRetryer
			if got := r.ShouldRetry(tt.req); got != tt.want {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}
