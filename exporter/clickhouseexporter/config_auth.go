// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"context"
	"errors"
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

// applyAuth attaches GetJWT from the configured authenticator for both native and HTTPS.
// GetJWT is used for HTTPS as well so the driver does not combine bearer auth with
// X-ClickHouse-User / X-ClickHouse-SSL-Certificate-Auth headers.
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

	opt.GetJWT = func(ctx context.Context) (string, error) {
		return tokenFromHTTPAuth(ctx, httpAuth)
	}

	// Fail at Start if the authenticator cannot produce a Bearer JWT.
	if _, err := opt.GetJWT(ctx); err != nil {
		return err
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

	if _, err := rt.RoundTrip(req); err != nil {
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
		return "", errors.New("authenticator did not return an authorization token")
	}

	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", fmt.Errorf("authorization header must use Bearer scheme, got %q", scheme)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errors.New("authenticator returned an empty token")
	}
	return token, nil
}
