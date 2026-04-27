// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"golang.org/x/oauth2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

// extensionTokenSourceProvider is the preferred interface for extensions that
// supply OAuth tokens. oauth2clientauthextension implements this directly.
type extensionTokenSourceProvider interface {
	TokenSource(context.Context) (oauth2.TokenSource, error)
}

// contextTokenSource is the duck-typed interface used by #41873.
// We detect it as a fallback for extensions that implement Token() but not TokenSource().
// Note: this interface was never exported or documented — it existed only in franz_client.go.
// Supporting it here is a best-effort compat shim, not a published contract.
type contextTokenSource interface {
	Token(context.Context) (*oauth2.Token, error)
}

// contextTokenSourceBridge bridges contextTokenSource → oauth2.TokenSource (no-context Token()).
type contextTokenSourceBridge struct {
	// ctx is the component's start-time context (with cancellation removed),
	// not the per-request auth callback context. This is intentional — using a
	// request-scoped context could cause token refresh to fail if that context is
	// cancelled mid-flight.
	ctx   context.Context
	inner contextTokenSource
}

func (b *contextTokenSourceBridge) Token() (*oauth2.Token, error) {
	return b.inner.Token(b.ctx)
}

// resolveOAuthBearerProvider eagerly resolves the configured OAUTHBEARER token source
// into a TokenProvider. ctx should be context.WithoutCancel(startCtx) so the resulting
// provider lives for the component's lifetime regardless of the startup context's deadline.
func resolveOAuthBearerProvider(
	ctx context.Context,
	cfg *configkafka.SASLConfig,
	host component.Host,
) (TokenProvider, error) {
	if cfg.OAuthBearerTokenFile != "" {
		return NewFileTokenProvider(cfg.OAuthBearerTokenFile)
	}
	ext, ok := host.GetExtensions()[cfg.OAuthBearerTokenSource]
	if !ok {
		return nil, fmt.Errorf("oauth token source extension %q not found", cfg.OAuthBearerTokenSource)
	}
	switch e := ext.(type) {
	case extensionTokenSourceProvider:
		ts, err := e.TokenSource(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain token source from extension %q: %w", cfg.OAuthBearerTokenSource, err)
		}
		return NewTokenSourceProvider(ts)
	case contextTokenSource:
		// Legacy compat: extension implements the duck-type from #41873.
		// Wrap with ReuseTokenSource to avoid a network round-trip on every auth handshake
		// for extensions that don't cache internally.
		wrapped := oauth2.ReuseTokenSource(nil, &contextTokenSourceBridge{ctx: ctx, inner: e})
		return NewTokenSourceProvider(wrapped)
	default:
		return nil, fmt.Errorf("extension %q does not implement TokenSource(context.Context) (oauth2.TokenSource, error)", cfg.OAuthBearerTokenSource)
	}
}
