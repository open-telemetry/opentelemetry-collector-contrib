// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"

import (
	"context"
	"errors"
	"net/http"
)

type contextKey int

const (
	clientContextKey            contextKey = iota
	failOnMissingMetadataCtxKey contextKey = iota
)

// ContextWithClient returns a new context.Context with the provided *http.Client stored as a value.
func ContextWithClient(ctx context.Context, client *http.Client) context.Context {
	return context.WithValue(ctx, clientContextKey, client)
}

// ContextWithFailOnMissingMetadata returns a new context with the failOnMissingMetadata flag stored as a value.
func ContextWithFailOnMissingMetadata(ctx context.Context, fail bool) context.Context {
	return context.WithValue(ctx, failOnMissingMetadataCtxKey, fail)
}

// FailOnMissingMetadataFromContext retrieves the failOnMissingMetadata flag from the context.
// Returns false if the value is not present (backward-compatible default).
func FailOnMissingMetadataFromContext(ctx context.Context) bool {
	v := ctx.Value(failOnMissingMetadataCtxKey)
	if v == nil {
		return false
	}
	if fail, ok := v.(bool); ok {
		return fail
	}
	return false
}

// ClientFromContext attempts to extract an *http.Client from the provided context.Context.
func ClientFromContext(ctx context.Context) (*http.Client, error) {
	v := ctx.Value(clientContextKey)
	if v == nil {
		return nil, errors.New("no http.Client in context")
	}
	var c *http.Client
	var ok bool
	if c, ok = v.(*http.Client); !ok {
		return nil, errors.New("invalid value found in context")
	}

	return c, nil
}
