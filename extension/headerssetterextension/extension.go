// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package headerssetterextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
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
	_ extension.Extension             = (*headerSetterExtension)(nil)
	_ extensionauth.HTTPClient        = (*headerSetterExtension)(nil)
	_ extensionauth.GRPCClient        = (*headerSetterExtension)(nil)
	_ extensioncapabilities.Dependent = (*headerSetterExtension)(nil)
)

type headerSetterExtension struct {
	component.StartFunc
	component.ShutdownFunc

	headers        []header
	additionalAuth *component.ID
	host           component.Host
}

// Dependencies implements extensioncapabilities.Dependent.
func (h *headerSetterExtension) Dependencies() []component.ID {
	if h.additionalAuth == nil {
		return nil
	}
	return []component.ID{*h.additionalAuth}
}

// Start stores the host for later use in getting the additional auth extension.
func (h *headerSetterExtension) Start(_ context.Context, host component.Host) error {
	h.host = host
	return nil
}

// getAdditionalAuthExtension retrieves the configured additional auth extension if present.
// Returns nil if no additional auth is configured.
func (h *headerSetterExtension) getAdditionalAuthExtension() (component.Component, error) {
	if h.additionalAuth == nil || h.host == nil {
		return nil, nil
	}

	ext := h.host.GetExtensions()[*h.additionalAuth]
	if ext == nil {
		return nil, fmt.Errorf("auth extension %v not found", h.additionalAuth)
	}

	return ext, nil
}

// PerRPCCredentials implements extensionauth.GRPCClient.
func (h *headerSetterExtension) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	var baseCredentials credentials.PerRPCCredentials

	// If additional_auth is configured, chain with it first
	ext, err := h.getAdditionalAuthExtension()
	if err != nil {
		return nil, err
	}

	if ext != nil {
		if grpcClient, ok := ext.(extensionauth.GRPCClient); ok {
			baseCredentials, err = grpcClient.PerRPCCredentials()
			if err != nil {
				return nil, fmt.Errorf("failed to get PerRPCCredentials from %v: %w", h.additionalAuth, err)
			}
		}
	}

	return &headersPerRPC{
		headers:         h.headers,
		baseCredentials: baseCredentials,
	}, nil
}

// RoundTripper implements extensionauth.HTTPClient.
func (h *headerSetterExtension) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	// If additional_auth is configured, chain with it first
	baseRT := base

	ext, err := h.getAdditionalAuthExtension()
	if err != nil {
		return nil, err
	}

	if ext != nil {
		// Check if it implements HTTPClient
		if httpClient, ok := ext.(extensionauth.HTTPClient); ok {
			baseRT, err = httpClient.RoundTripper(base)
			if err != nil {
				return nil, fmt.Errorf("failed to get RoundTripper from %v: %w", h.additionalAuth, err)
			}
		}
	}

	// Now wrap with our headers
	return &headersRoundTripper{
		base:    baseRT,
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

	ext := &headerSetterExtension{
		headers:        headers,
		additionalAuth: cfg.AdditionalAuth,
	}

	// Enable Start method if additional_auth is configured
	if cfg.AdditionalAuth != nil {
		ext.StartFunc = ext.Start
	}

	return ext, nil
}

// headersPerRPC is a gRPC credentials.PerRPCCredentials implementation sets
// headers with values extracted from provided sources.
type headersPerRPC struct {
	headers         []header
	baseCredentials credentials.PerRPCCredentials
}

// GetRequestMetadata returns the request metadata to be used with the RPC.
func (h *headersPerRPC) GetRequestMetadata(
	ctx context.Context,
	uri ...string,
) (map[string]string, error) {
	// Start with base credentials if available
	metadata := make(map[string]string)

	if h.baseCredentials != nil {
		baseMetadata, err := h.baseCredentials.GetRequestMetadata(ctx, uri...)
		if err != nil {
			return nil, err
		}
		// Copy base metadata
		maps.Copy(metadata, baseMetadata)
	}

	// Now apply our headers on top
	for _, header := range h.headers {
		value, err := header.source.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to determine the source: %w", err)
		}
		header.action.ApplyOnMetadata(metadata, value)
	}
	return metadata, nil
}

// RequireTransportSecurity returns whether transport security is required.
// If chained with another auth extension, delegate to it.
func (h *headersPerRPC) RequireTransportSecurity() bool {
	if h.baseCredentials != nil {
		return h.baseCredentials.RequireTransportSecurity()
	}
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
