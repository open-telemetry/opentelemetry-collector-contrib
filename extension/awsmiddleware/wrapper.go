// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsmiddleware // import "github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/smithy-go/middleware"
	"github.com/aws/smithy-go/transport/http"
	"github.com/google/uuid"
)

type key struct{}

var requestID key

func namedRequestHandler(handler RequestHandler) request.NamedHandler {
	return request.NamedHandler{Name: handler.ID(), Fn: func(r *request.Request) {
		ctx, id := setID(r.Context())
		r.SetContext(ctx)
		handler.HandleRequest(id, r.HTTPRequest)
	}}
}

func namedResponseHandler(handler ResponseHandler) request.NamedHandler {
	return request.NamedHandler{Name: handler.ID(), Fn: func(r *request.Request) {
		id, _ := getID(r.Context())
		handler.HandleResponse(id, r.HTTPResponse)
	}}
}

type requestMiddleware struct {
	RequestHandler
}

var _ middleware.BuildMiddleware = (*requestMiddleware)(nil)

func (r requestMiddleware) HandleBuild(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (out middleware.BuildOutput, metadata middleware.Metadata, err error) {
	req, ok := in.Request.(*http.Request)
	if ok {
		var id string
		ctx, id = setID(ctx)
		r.HandleRequest(id, req.Request)
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
		id, _ := getID(ctx)
		r.HandleResponse(id, res.Response)
	}
	return
}

func withDeserializeOption(rmw *responseMiddleware, position middleware.RelativePosition) func(stack *middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Add(rmw, position)
	}
}

func setID(ctx context.Context) (context.Context, string) {
	id, ok := getID(ctx)
	if !ok {
		id = uuid.NewString()
		return context.WithValue(ctx, requestID, id), id
	}
	return ctx, id
}

func getID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestID).(string)
	return id, ok
}
