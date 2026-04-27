// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

// extensionsHost is a minimal component.Host that returns a fixed set of extensions.
type extensionsHost map[component.ID]component.Component

func (h extensionsHost) GetExtensions() map[component.ID]component.Component { return h }

type nonTokenSourceExtension struct{}

func (nonTokenSourceExtension) Start(context.Context, component.Host) error { return nil }
func (nonTokenSourceExtension) Shutdown(context.Context) error              { return nil }

type errorTokenSourceExtension struct{}

func (errorTokenSourceExtension) Start(context.Context, component.Host) error { return nil }
func (errorTokenSourceExtension) Shutdown(context.Context) error              { return nil }
func (errorTokenSourceExtension) TokenSource(context.Context) (oauth2.TokenSource, error) {
	return nil, errors.New("boom")
}

func newOAuthObserverConfig() *Config {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	extID := component.MustNewID("oauth2client")
	cfg.Authentication.SASL = &configkafka.SASLConfig{
		Mechanism:              kafka.OAUTHBEARER,
		OAuthBearerTokenSource: extID,
	}
	return cfg
}

func newTestObserver(t *testing.T, cfg *Config) *kafkaTopicsObserver {
	t.Helper()
	ext, err := newObserver(
		zap.NewNop(),
		cfg,
		kafka.NewSaramaClusterAdminClient,
	)
	require.NoError(t, err)
	return ext.(*kafkaTopicsObserver)
}

func TestStart_OAuthExtensionMissing(t *testing.T) {
	obs := newTestObserver(t, newOAuthObserverConfig())

	err := obs.Start(t.Context(), extensionsHost{})
	require.Error(t, err)
	require.ErrorContains(t, err, "oauth token source extension \"oauth2client\" not found")
}

func TestStart_OAuthExtensionWrongType(t *testing.T) {
	cfg := newOAuthObserverConfig()
	obs := newTestObserver(t, cfg)
	extID := cfg.Authentication.SASL.OAuthBearerTokenSource

	err := obs.Start(t.Context(), extensionsHost{extID: nonTokenSourceExtension{}})
	require.Error(t, err)
	require.ErrorContains(t, err, "does not implement TokenSource(context.Context) (oauth2.TokenSource, error)")
}

func TestStart_OAuthExtensionError(t *testing.T) {
	cfg := newOAuthObserverConfig()
	obs := newTestObserver(t, cfg)
	extID := cfg.Authentication.SASL.OAuthBearerTokenSource

	err := obs.Start(t.Context(), extensionsHost{extID: errorTokenSourceExtension{}})
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to obtain token source from extension \"oauth2client\"")
	require.ErrorContains(t, err, "boom")
}
