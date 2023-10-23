// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsmiddleware // import "github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"

import (
	"context"

	sdkmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/smithy-go/middleware"
	"github.com/aws/smithy-go/transport/http"
	"github.com/google/uuid"
)

type (
	requestIDKey     struct{}
	operationNameKey struct{}
)

func namedRequestHandler(handler RequestHandler) request.NamedHandler {
	return request.NamedHandler{Name: handler.ID(), Fn: func(r *request.Request) {
		ctx := mustRequestID(r.Context())
		ctx = setOperationName(ctx, r.Operation.Name)
		r.SetContext(ctx)
		handler.HandleRequest(ctx, r.HTTPRequest)
	}}
}

func namedResponseHandler(handler ResponseHandler) request.NamedHandler {
	return request.NamedHandler{Name: handler.ID(), Fn: func(r *request.Request) {
		handler.HandleResponse(r.Context(), r.HTTPResponse)
	}}
}

type requestMiddleware struct {
	RequestHandler
}

var _ middleware.BuildMiddleware = (*requestMiddleware)(nil)

func (r requestMiddleware) HandleBuild(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (out middleware.BuildOutput, metadata middleware.Metadata, err error) {
	req, ok := in.Request.(*http.Request)
	if ok {
		ctx = mustRequestID(ctx)
		ctx = setOperationName(ctx, sdkmiddleware.GetOperationName(ctx))
		r.HandleRequest(ctx, req.Request)
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
		r.HandleResponse(ctx, res.Response)
	}
	return
}

func withDeserializeOption(rmw *responseMiddleware, position middleware.RelativePosition) func(stack *middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Deserialize.Add(rmw, position)
	}
}

func mustRequestID(ctx context.Context) context.Context {
	requestID := GetRequestID(ctx)
	if requestID != "" {
		return ctx
	}
	return setRequestID(ctx, uuid.NewString())
}

func setRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func setOperationName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, operationNameKey{}, name)
}

// GetRequestID retrieves the generated request ID from the context.
func GetRequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

// GetOperationName retrieves the service operation metadata from the context.
func GetOperationName(ctx context.Context) string {
	operationName, _ := ctx.Value(operationNameKey{}).(string)
	return operationName
}
