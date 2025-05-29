// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
)

func TestWithStructuredLogHeader(t *testing.T) {
	stack := middleware.NewStack("with structured log", smithyhttp.NewStackRequest)
	if err := stack.Serialize.Add(&withStructuredLogHeader{}, middleware.Before); err != nil {
		t.Fatal(err)
	}

	mockHandler := middleware.HandlerFunc(func(ctx context.Context, in any) (any, middleware.Metadata, error) {
		req := in.(*smithyhttp.Request).Build(ctx)

		assert.Equal(t, "json/emf", req.Header.Get("x-amzn-logs-format"))

		return &smithyhttp.Response{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
			},
		}, middleware.Metadata{}, nil
	})

	handler := middleware.DecorateHandler(mockHandler, stack)
	if _, _, err := handler.Handle(context.Background(), nil); err != nil {
		t.Error(err)
	}
}
