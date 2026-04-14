// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"golang.org/x/oauth2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

type nonTokenSourceExtension struct{}

func (nonTokenSourceExtension) Start(context.Context, component.Host) error { return nil }
func (nonTokenSourceExtension) Shutdown(context.Context) error              { return nil }

type errorTokenSourceExtension struct{}

func (errorTokenSourceExtension) Start(context.Context, component.Host) error { return nil }
func (errorTokenSourceExtension) Shutdown(context.Context) error              { return nil }
func (errorTokenSourceExtension) TokenSource(context.Context) (oauth2.TokenSource, error) {
	return nil, errors.New("boom")
}

func TestFranzConsumerStart_OAuthTokenSourceExtensionMissing(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	extID := component.MustNewID("oauth2client")
	cfg.Authentication.SASL = &configkafka.SASLConfig{
		Mechanism:              kafka.OAUTHBEARER,
		OAuthBearerTokenSource: extID,
	}

	consumer, err := newFranzKafkaConsumer(cfg, receivertest.NewNopSettings(metadata.Type), []string{"topic"}, nil, nil)
	require.NoError(t, err)

	err = consumer.Start(t.Context(), extensionsHost{})
	require.Error(t, err)
	require.ErrorContains(t, err, "oauth token source extension \"oauth2client\" not found")
}

func TestFranzConsumerStart_OAuthTokenSourceExtensionWrongType(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	extID := component.MustNewID("oauth2client")
	cfg.Authentication.SASL = &configkafka.SASLConfig{
		Mechanism:              kafka.OAUTHBEARER,
		OAuthBearerTokenSource: extID,
	}

	consumer, err := newFranzKafkaConsumer(cfg, receivertest.NewNopSettings(metadata.Type), []string{"topic"}, nil, nil)
	require.NoError(t, err)

	err = consumer.Start(t.Context(), extensionsHost{extID: nonTokenSourceExtension{}})
	require.Error(t, err)
	require.ErrorContains(t, err, "does not implement TokenSource(context.Context) (oauth2.TokenSource, error)")
}

func TestFranzConsumerStart_OAuthTokenSourceExtensionError(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	extID := component.MustNewID("oauth2client")
	cfg.Authentication.SASL = &configkafka.SASLConfig{
		Mechanism:              kafka.OAUTHBEARER,
		OAuthBearerTokenSource: extID,
	}

	consumer, err := newFranzKafkaConsumer(cfg, receivertest.NewNopSettings(metadata.Type), []string{"topic"}, nil, nil)
	require.NoError(t, err)

	err = consumer.Start(t.Context(), extensionsHost{extID: errorTokenSourceExtension{}})
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to obtain token source from extension \"oauth2client\"")
	require.ErrorContains(t, err, "boom")
}
