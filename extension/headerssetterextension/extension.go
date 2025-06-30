// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package headerssetterextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension/internal/action"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension/internal/source"
)

type header struct {
	action action.Action
	source source.Source
}

var (
	_ extension.Extension      = (*headerSetterExtension)(nil)
	_ extensionauth.HTTPClient = (*headerSetterExtension)(nil)
	_ extensionauth.GRPCClient = (*headerSetterExtension)(nil)
)

type headerSetterExtension struct {
	component.StartFunc
	component.ShutdownFunc

	headers []header
}

// PerRPCCredentials implements extensionauth.GRPCClient.
func (h *headerSetterExtension) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return &headersPerRPC{headers: h.headers}, nil
}

// RoundTripper implements extensionauth.HTTPClient.
func (h *headerSetterExtension) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return &headersRoundTripper{
		base:    base,
		headers: h.headers,
	}, nil
}

func newHeadersSetterExtension(cfg *Config, logger *zap.Logger) (*headerSetterExtension, error) {
	if cfg == nil {
		return nil, errors.New("extension configuration is not provided")
	}

	headers := make([]header, 0, len(cfg.HeadersConfig))
	for _, h := range cfg.HeadersConfig {
		var s source.Source
		switch {
		case h.Value != nil:
			s = &source.StaticSource{
				Value: *h.Value,
			}
		case h.FromAttribute != nil:
			defaultValue := ""
			if h.DefaultValue != nil {
				defaultValue = string(*h.DefaultValue)
			}
			s = &source.AttributeSource{
				Key:          *h.FromAttribute,
				DefaultValue: defaultValue,
			}
		case h.FromContext != nil:
			defaultValue := ""
			if h.DefaultValue != nil {
				defaultValue = string(*h.DefaultValue)
			}
			s = &source.ContextSource{
				Key:          *h.FromContext,
				DefaultValue: defaultValue,
			}
		}

		var a action.Action
		switch h.Action {
		case INSERT:
			a = action.Insert{Key: *h.Key}
		case UPSERT:
			a = action.Upsert{Key: *h.Key}
		case UPDATE:
			a = action.Update{Key: *h.Key}
		case DELETE:
			a = action.Delete{Key: *h.Key}
		default:
			a = action.Upsert{Key: *h.Key}
			logger.Warn("The action was not provided, using 'upsert'." +
				" In future versions, we'll require this to be explicitly set")
		}
		headers = append(headers, header{action: a, source: s})
	}

	return &headerSetterExtension{headers: headers}, nil
}

// headersPerRPC is a gRPC credentials.PerRPCCredentials implementation sets
// headers with values extracted from provided sources.
type headersPerRPC struct {
	headers []header
}

// GetRequestMetadata returns the request metadata to be used with the RPC.
func (h *headersPerRPC) GetRequestMetadata(
	ctx context.Context,
	_ ...string,
) (map[string]string, error) {
	metadata := make(map[string]string, len(h.headers))
	for _, header := range h.headers {
		value, err := header.source.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to determine the source: %w", err)
		}
		header.action.ApplyOnMetadata(metadata, value)
	}
	return metadata, nil
}

// RequireTransportSecurity always returns false for this implementation.
// The header setter is not sending auth data, so it should not require
// a transport security.
func (h *headersPerRPC) RequireTransportSecurity() bool {
	return false
}

// headersRoundTripper intercepts downstream requests and sets headers with
// values extracted from configured sources.
type headersRoundTripper struct {
	base    http.RoundTripper
	headers []header
}

// RoundTrip copies the original request and sets headers of the new requests
// with values extracted from configured sources.
func (h *headersRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	if req2.Header == nil {
		req2.Header = make(http.Header)
	}
	for _, header := range h.headers {
		value, err := header.source.Get(req.Context())
		if err != nil {
			return nil, fmt.Errorf("failed to determine the source: %w", err)
		}
		header.action.ApplyOnHeaders(req2.Header, value)
	}
	return h.base.RoundTrip(req2)
}
