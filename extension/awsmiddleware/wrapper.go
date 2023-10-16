// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsmiddleware // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsmiddleware"

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/smithy-go/middleware"
	"github.com/aws/smithy-go/transport/http"
)

func namedRequestHandler(handler RequestHandler) request.NamedHandler {
	return request.NamedHandler{Name: handler.ID(), Fn: func(r *request.Request) {
		handler.HandleRequest(r.HTTPRequest)
	}}
}

func namedResponseHandler(handler ResponseHandler) request.NamedHandler {
	return request.NamedHandler{Name: handler.ID(), Fn: func(r *request.Request) {
		handler.HandleResponse(r.HTTPResponse)
	}}
}

type requestMiddleware struct {
	RequestHandler
}

var _ middleware.BuildMiddleware = (*requestMiddleware)(nil)

func (r requestMiddleware) HandleBuild(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (out middleware.BuildOutput, metadata middleware.Metadata, err error) {
	req, ok := in.Request.(*http.Request)
	if ok {
		r.HandleRequest(req.Request)
	}
	return next.HandleBuild(ctx, in)
}

func withBuildOption(rmw *requestMiddleware, position middleware.RelativePosition) func(stack *middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Build.Add(rmw, position)
	}
}

type responseMiddleware struct {
	ResponseHandler
}

var _ middleware.DeserializeMiddleware = (*responseMiddleware)(nil)

func (r responseMiddleware) HandleDeserialize(ctx context.Context, in middleware.DeserializeInput, next middleware.DeserializeHandler) (out middleware.DeserializeOutput, metadata middleware.Metadata, err error) {
	out, metadata, err = next.HandleDeserialize(ctx, in)
	res, ok := out.RawResponse.(*http.Response)
	if ok {
		r.HandleResponse(res.Response)
	}
	return
}

func withDeserializeOption(rmw *responseMiddleware, position middleware.RelativePosition) func(stack *middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Add(rmw, position)
	}
}
