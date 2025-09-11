// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"

import (
	"context"
	"errors"
	"net/http"
)

type contextKey int

const clientContextKey contextKey = iota

// ContextWithClient returns a new context.Context with the provided *http.Client stored as a value.
func ContextWithClient(ctx context.Context, client *http.Client) context.Context {
	return context.WithValue(ctx, clientContextKey, client)
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
