// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package handler // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs/handler"

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type withStructuredLogHeader struct{}

var _ middleware.SerializeMiddleware = (*withStructuredLogHeader)(nil)

func (*withStructuredLogHeader) ID() string {
	return "RequestStructuredLogHandler"
}

func (*withStructuredLogHeader) HandleSerialize(ctx context.Context, in middleware.SerializeInput, next middleware.SerializeHandler) (middleware.SerializeOutput, middleware.Metadata, error) {
	req, ok := in.Request.(*smithyhttp.Request)
	if !ok {
		return middleware.SerializeOutput{}, middleware.Metadata{}, fmt.Errorf("unrecognized transport type: %T", in.Request)
	}
	req.Header.Set("x-amzn-logs-format", "json/emf")
	return next.HandleSerialize(ctx, in)
}

// WithStructuredLogHeader sets the `x-amzn-logs-format` header to `json/emf`
func WithStructuredLogHeader(pos middleware.RelativePosition) func(options *cloudwatchlogs.Options) {
	return func(o *cloudwatchlogs.Options) {
		o.APIOptions = append(o.APIOptions, func(s *middleware.Stack) error {
			return s.Serialize.Add(&withStructuredLogHeader{}, pos)
		})
	}
}
