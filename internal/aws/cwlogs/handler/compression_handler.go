// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package handler // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs/handler"

import (
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/smithy-go/middleware"
	smithyrequestcompression "github.com/aws/smithy-go/private/requestcompression"
)

// WithRequestCompression registers gzip request-body compression on the
// CloudWatch Logs client's middleware stack.
func WithRequestCompression(disable bool, minSize int64) func(*cloudwatchlogs.Options) {
	return func(o *cloudwatchlogs.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			return smithyrequestcompression.AddRequestCompression(
				stack,
				disable,
				minSize,
				[]string{"gzip"},
			)
		})
	}
}
