// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensionauth"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
)

func (cfg *Config) openClickHouse(ctx context.Context, host component.Host) (driver.Conn, error) {
	opt, err := cfg.buildClickHouseOptions()
	if err != nil {
		return nil, err
	}
	if err := cfg.applyAuth(ctx, opt, host); err != nil {
		return nil, err
	}
	return internal.NewClickhouseClientFromOptions(opt, cfg.shouldCreateSchema())
}

// applyAuth attaches the configured authenticator. HTTP uses TransportFunc; native uses GetJWT.
func (cfg *Config) applyAuth(ctx context.Context, opt *clickhouse.Options, host component.Host) error {
	if !cfg.Auth.HasValue() {
		return nil
	}
	if host == nil {
		return errAuthRequiresHost
	}
	if opt.TLS == nil {
		return errAuthRequiresTLS
	}

	httpAuth, err := cfg.Auth.Get().GetHTTPClientAuthenticator(ctx, host.GetExtensions())
	if err != nil {
		return fmt.Errorf("failed to resolve auth extension: %w", err)
	}

	if opt.Protocol == clickhouse.HTTP {
		opt.TransportFunc = func(base *http.Transport) (http.RoundTripper, error) {
			return httpAuth.RoundTripper(base)
		}
		return nil
	}

	opt.GetJWT = func(ctx context.Context) (string, error) {
		return tokenFromHTTPAuth(ctx, httpAuth)
	}
	return nil
}

func tokenFromHTTPAuth(ctx context.Context, auth extensionauth.HTTPClient) (string, error) {
	capture := &headerCaptureRoundTripper{}
	rt, err := auth.RoundTripper(capture)
	if err != nil {
		return "", fmt.Errorf("failed to create auth round tripper: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "http://localhost", http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to build auth request: %w", err)
	}

	if _, err := rt.RoundTrip(req); err != nil && capture.authorization == "" {
		return "", fmt.Errorf("failed to obtain auth token: %w", err)
	}

	return jwtFromAuthorization(capture.authorization)
}

type headerCaptureRoundTripper struct {
	authorization string
}

func (h *headerCaptureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	h.authorization = req.Header.Get("Authorization")
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
}

func jwtFromAuthorization(header string) (string, error) {
	if header == "" {
		return "", fmt.Errorf("authenticator did not return an authorization token")
	}

	token := header
	if after, ok := strings.CutPrefix(header, "Bearer "); ok {
		token = after
	} else if after, ok := strings.CutPrefix(header, "bearer "); ok {
		token = after
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("authenticator returned an empty token")
	}
	return token, nil
}
