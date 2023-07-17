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
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/request"
)

// IMDSRetryer this must implement request.Retryer
// not sure how to make in a test context not to retry
// this seems to only be an issue on mac
// windows and linux do not have this issue
// in text context do not try to retry
// this causes timeout failures for mac unit tests
// currently we set the var to nil in tests to mock
var IMDSRetryer request.Retryer = newIMDSRetryer()

type iMDSRetryer struct {
	client.DefaultRetryer
}

// newIMDSRetryer allows us to retry imds errors
// 2 imds calls 1 for hostname 1 for doc
// 2 calls 1 for imdsv2 1 for imdsv1
// 2 seconds 4 seconds 8 seconds 16 seconds 32 seconds = 1 minute 2 seconds
// total is 4 minutes 8 seconds
// random jitter is applied of half the retry time
// max retry total time is 6 minutes 12 seconds
// min retry total time is 2 minutes 4 seconds
func newIMDSRetryer() iMDSRetryer {
	return iMDSRetryer{
		DefaultRetryer: client.DefaultRetryer{
			NumMaxRetries: 5,
			MinRetryDelay: 2 * time.Second,
		},
	}
}

func (r iMDSRetryer) ShouldRetry(req *request.Request) bool {
	// there is no enum of error codes
	// EC2MetadataError is not retryable by default
	// Fallback to SDK's built in retry rules
	shouldRetry := false
	if awsError, ok := req.Error.(awserr.Error); r.DefaultRetryer.ShouldRetry(req) || (ok && awsError != nil && awsError.Code() == "EC2MetadataError") {
		shouldRetry = true
	}
	return shouldRetry
}
