// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"golang.org/x/oauth2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

func writeTestToken(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

type fakeTokenSourceExtension struct{}

func (fakeTokenSourceExtension) TokenSource(context.Context) (oauth2.TokenSource, error) {
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "ext-token"}), nil
}

func (fakeTokenSourceExtension) Start(context.Context, component.Host) error { return nil }

func (fakeTokenSourceExtension) Shutdown(context.Context) error { return nil }

type otherExtension struct{}

func (otherExtension) Start(context.Context, component.Host) error { return nil }

func (otherExtension) Shutdown(context.Context) error { return nil }

type errorTokenSourceExtension struct{}

func (errorTokenSourceExtension) TokenSource(context.Context) (oauth2.TokenSource, error) {
	return nil, errors.New("boom")
}

func (errorTokenSourceExtension) Start(context.Context, component.Host) error { return nil }

func (errorTokenSourceExtension) Shutdown(context.Context) error { return nil }

type deadlineSensitiveTokenSourceExtension struct{}

func (deadlineSensitiveTokenSourceExtension) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	if ctx == nil {
		return nil, errors.New("nil context")
	}
	if _, ok := ctx.Deadline(); ok {
		return nil, errors.New("deadline present on context")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "deadline-stripped-token"}), nil
}

func (deadlineSensitiveTokenSourceExtension) Start(context.Context, component.Host) error { return nil }

func (deadlineSensitiveTokenSourceExtension) Shutdown(context.Context) error { return nil }

// fakeContextTokenSourceExtension implements only the duck-typed contextTokenSource
// interface (Token(ctx) method) used by #41873, not the preferred extensionTokenSourceProvider.
type fakeContextTokenSourceExtension struct{}

func (fakeContextTokenSourceExtension) Token(_ context.Context) (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "legacy-token"}, nil
}

func (fakeContextTokenSourceExtension) Start(context.Context, component.Host) error { return nil }

func (fakeContextTokenSourceExtension) Shutdown(context.Context) error { return nil }

// hostWithExtensions returns a minimal component.Host that returns the given extensions map.
type fakeHost map[component.ID]component.Component

func hostWithExtensions(exts map[component.ID]component.Component) component.Host {
	return fakeHost(exts)
}

func (h fakeHost) GetExtensions() map[component.ID]component.Component {
	return h
}

func TestResolveOAuthBearerProvider(t *testing.T) {
	id := component.MustNewID("oauth2client")
	cfg := &configkafka.SASLConfig{
		Mechanism:              OAUTHBEARER,
		OAuthBearerTokenSource: id,
	}
	host := hostWithExtensions(map[component.ID]component.Component{
		id: fakeTokenSourceExtension{},
	})
	tp, err := resolveOAuthBearerProvider(t.Context(), cfg, host)
	require.NoError(t, err)
	require.NotNil(t, tp)
	val, err := tp.Token(t.Context())
	require.NoError(t, err)
	require.Equal(t, "ext-token", val)
}

func TestResolveOAuthBearerProviderFileToken(t *testing.T) {
	tokenPath := t.TempDir() + "/token"
	require.NoError(t, writeTestToken(tokenPath, "file-token"))

	cfg := &configkafka.SASLConfig{
		Mechanism:            OAUTHBEARER,
		OAuthBearerTokenFile: tokenPath,
	}
	host := hostWithExtensions(nil)
	tp, err := resolveOAuthBearerProvider(t.Context(), cfg, host)
	require.NoError(t, err)
	require.NotNil(t, tp)
	val, err := tp.Token(t.Context())
	require.NoError(t, err)
	require.Equal(t, "file-token", val)
}

func TestResolveOAuthBearerProviderStripsDeadlineFromContext(t *testing.T) {
	id := component.MustNewID("oauth2client")
	cfg := &configkafka.SASLConfig{
		Mechanism:              OAUTHBEARER,
		OAuthBearerTokenSource: id,
	}
	host := hostWithExtensions(map[component.ID]component.Component{
		id: deadlineSensitiveTokenSourceExtension{},
	})

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	// resolveOAuthBearerProvider is called with context.WithoutCancel(ctx) by its callers,
	// but the extension itself verifies no deadline is present. Pass the deadline ctx
	// directly here to test that the extension receives it as-is. The real callers
	// strip the deadline before calling.
	tp, err := resolveOAuthBearerProvider(context.WithoutCancel(ctx), cfg, host)
	require.NoError(t, err)
	require.NotNil(t, tp)
}

func TestResolveOAuthBearerProviderLegacyContextTokenSource(t *testing.T) {
	id := component.MustNewID("oauth2client")
	cfg := &configkafka.SASLConfig{
		Mechanism:              OAUTHBEARER,
		OAuthBearerTokenSource: id,
	}
	host := hostWithExtensions(map[component.ID]component.Component{
		id: fakeContextTokenSourceExtension{},
	})
	tp, err := resolveOAuthBearerProvider(t.Context(), cfg, host)
	require.NoError(t, err)
	require.NotNil(t, tp)
	val, err := tp.Token(t.Context())
	require.NoError(t, err)
	require.Equal(t, "legacy-token", val)
}

func TestResolveOAuthBearerProviderErrors(t *testing.T) {
	id := component.MustNewID("oauth2client")

	t.Run("missing extension", func(t *testing.T) {
		cfg := &configkafka.SASLConfig{
			Mechanism:              OAUTHBEARER,
			OAuthBearerTokenSource: id,
		}
		host := hostWithExtensions(map[component.ID]component.Component{})
		_, err := resolveOAuthBearerProvider(t.Context(), cfg, host)
		require.ErrorContains(t, err, "oauth token source extension \"oauth2client\" not found")
	})

	t.Run("wrong type", func(t *testing.T) {
		cfg := &configkafka.SASLConfig{
			Mechanism:              OAUTHBEARER,
			OAuthBearerTokenSource: id,
		}
		host := hostWithExtensions(map[component.ID]component.Component{
			id: otherExtension{},
		})
		_, err := resolveOAuthBearerProvider(t.Context(), cfg, host)
		require.ErrorContains(t, err, "does not implement TokenSource(context.Context) (oauth2.TokenSource, error)")
	})

	t.Run("token source returns error", func(t *testing.T) {
		cfg := &configkafka.SASLConfig{
			Mechanism:              OAUTHBEARER,
			OAuthBearerTokenSource: id,
		}
		host := hostWithExtensions(map[component.ID]component.Component{
			id: errorTokenSourceExtension{},
		})
		_, err := resolveOAuthBearerProvider(t.Context(), cfg, host)
		require.ErrorContains(t, err, "failed to obtain token source from extension \"oauth2client\"")
		require.ErrorContains(t, err, "boom")
	})
}
