// Copyright 2019, OpenTelemetry Authors
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

package awsxrayexporter

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/xray"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

var collectorDistribution = "opentelemetry-collector-contrib"

// XRay defines X-Ray api call structure.
type XRay interface {
	PutTraceSegments(input *xray.PutTraceSegmentsInput) (*xray.PutTraceSegmentsOutput, error)
	PutTelemetryRecords(input *xray.PutTelemetryRecordsInput) (*xray.PutTelemetryRecordsOutput, error)
}

// XRayClient represents X-Ray client.
type XRayClient struct {
	xRay *xray.XRay
}

// PutTraceSegments makes PutTraceSegments api call on X-Ray client.
func (c *XRayClient) PutTraceSegments(input *xray.PutTraceSegmentsInput) (*xray.PutTraceSegmentsOutput, error) {
	return c.xRay.PutTraceSegments(input)
}

// PutTelemetryRecords makes PutTelemetryRecords api call on X-Ray client.
func (c *XRayClient) PutTelemetryRecords(input *xray.PutTelemetryRecordsInput) (*xray.PutTelemetryRecordsOutput, error) {
	return c.xRay.PutTelemetryRecords(input)
}

// newXRay creates a new instance of the XRay client with a aws configuration and session .
func newXRay(logger *zap.Logger, awsConfig *aws.Config, startInfo component.ApplicationStartInfo, s *session.Session) XRay {
	x := xray.New(s, awsConfig)
	logger.Debug("Using Endpoint: %s", zap.String("endpoint", x.Endpoint))

	x.Handlers.Build.PushBackNamed(request.NamedHandler{
		Name: "tracing.XRayVersionUserAgentHandler",
		Fn:   request.MakeAddToUserAgentHandler("xray", "1.0", os.Getenv("AWS_EXECUTION_ENV")),
	})

	x.Handlers.Build.PushFrontNamed(newCollectorUserAgentHandler(startInfo))

	x.Handlers.Sign.PushFrontNamed(request.NamedHandler{
		Name: "tracing.TimestampHandler",
		Fn: func(r *request.Request) {
			r.HTTPRequest.Header.Set("X-Amzn-Xray-Timestamp",
				strconv.FormatFloat(float64(time.Now().UnixNano())/float64(time.Second), 'f', 9, 64))
		},
	})

	return &XRayClient{
		xRay: x,
	}
}

// IsTimeoutError checks whether error is timeout error.
func IsTimeoutError(err error) bool {
	awsError, ok := err.(awserr.Error)
	if ok {
		if strings.Contains(awsError.Error(), "net/http: request canceled") {
			return true
		}
	}
	return false
}

func newCollectorUserAgentHandler(startInfo component.ApplicationStartInfo) request.NamedHandler {
	return request.NamedHandler{
		Name: "otel.collector.UserAgentHandler",
		Fn:   request.MakeAddToUserAgentHandler(collectorDistribution, startInfo.Version, startInfo.GitHash),
	}
}
