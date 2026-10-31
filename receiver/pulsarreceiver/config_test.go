// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix && !solaris

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.Equal(t, &Config{
		Topic:                 "otel-pulsar",
		Endpoint:              "pulsar://localhost:6500",
		ConsumerName:          "otel-collector",
		Subscription:          "otel-collector",
		Encoding:              defaultEncoding,
		TLSTrustCertsFilePath: "ca.pem",
		Authentication:        Authentication{TLS: configoptional.Some(TLS{CertFile: "cert.pem", KeyFile: "key.pem"})},
	},
		cfg,
	)
}

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	require.NoError(t, cfg.Validate())
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		expectedErr error
	}{
		{
			name: "valid config",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     defaultEncoding,
			},
		},
		{
			name: "valid with empty endpoint (defaults in clientOptions)",
			cfg: &Config{
				Endpoint:     "",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     defaultEncoding,
			},
		},
		{
			name: "valid with empty topic (defaults per signal in factory)",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "",
				Subscription: "test-subscription",
				Encoding:     defaultEncoding,
			},
		},
		{
			name: "missing subscription",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "",
				Encoding:     defaultEncoding,
			},
			expectedErr: errMissingSubscription,
		},
		{
			name: "empty encoding",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     "",
			},
			expectedErr: errUnsupportedEncoding,
		},
		{
			name: "unsupported encoding",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     "unsupported_format",
			},
			expectedErr: errUnsupportedEncoding,
		},
		{
			name: "supported encoding jaeger_proto",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     "jaeger_proto",
			},
		},
		{
			name: "supported encoding jaeger_json",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     "jaeger_json",
			},
		},
		{
			name: "supported encoding zipkin_proto",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     "zipkin_proto",
			},
		},
		{
			name: "supported encoding zipkin_json",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     "zipkin_json",
			},
		},
		{
			name: "supported encoding zipkin_thrift",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     "zipkin_thrift",
			},
		},
		{
			name: "valid with tls auth",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     defaultEncoding,
				Authentication: Authentication{
					TLS: configoptional.Some(TLS{CertFile: "cert.pem", KeyFile: "key.pem"}),
				},
			},
		},
		{
			name: "valid with token auth",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     defaultEncoding,
				Authentication: Authentication{
					Token: configoptional.Some(Token{Token: "token-value"}),
				},
			},
		},
		{
			name: "valid with oauth2 auth",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     defaultEncoding,
				Authentication: Authentication{
					OAuth2: configoptional.Some(OAuth2{IssuerURL: "https://auth.example.com", ClientID: "client-id"}),
				},
			},
		},
		{
			name: "valid with athenz auth",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     defaultEncoding,
				Authentication: Authentication{
					Athenz: configoptional.Some(Athenz{ProviderDomain: "example.com"}),
				},
			},
		},
		{
			name: "valid multiple auth tls and token (precedence preserved)",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     defaultEncoding,
				Authentication: Authentication{
					TLS:   configoptional.Some(TLS{CertFile: "cert.pem", KeyFile: "key.pem"}),
					Token: configoptional.Some(Token{Token: "token-value"}),
				},
			},
		},
		{
			name: "valid multiple auth token and oauth2 (precedence preserved)",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     defaultEncoding,
				Authentication: Authentication{
					Token:  configoptional.Some(Token{Token: "token-value"}),
					OAuth2: configoptional.Some(OAuth2{ClientID: "client-id"}),
				},
			},
		},
		{
			name: "valid multiple auth oauth2 and athenz (precedence preserved)",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     defaultEncoding,
				Authentication: Authentication{
					OAuth2: configoptional.Some(OAuth2{ClientID: "client-id"}),
					Athenz: configoptional.Some(Athenz{ProviderDomain: "example.com"}),
				},
			},
		},
		{
			name: "valid multiple auth tls and athenz (precedence preserved)",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     defaultEncoding,
				Authentication: Authentication{
					TLS:    configoptional.Some(TLS{CertFile: "cert.pem", KeyFile: "key.pem"}),
					Athenz: configoptional.Some(Athenz{ProviderDomain: "example.com"}),
				},
			},
		},
		{
			name: "valid multiple auth all (precedence preserved)",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "test-subscription",
				Encoding:     defaultEncoding,
				Authentication: Authentication{
					TLS:    configoptional.Some(TLS{CertFile: "cert.pem", KeyFile: "key.pem"}),
					Token:  configoptional.Some(Token{Token: "token-value"}),
					OAuth2: configoptional.Some(OAuth2{ClientID: "client-id"}),
					Athenz: configoptional.Some(Athenz{ProviderDomain: "example.com"}),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClientOptions(t *testing.T) {
	cfg := &Config{}
	opts := cfg.clientOptions()
	assert.Equal(t, defaultServiceURL, opts.URL)

	cfg.Endpoint = "pulsar://custom:6650"
	opts = cfg.clientOptions()
	assert.Equal(t, "pulsar://custom:6650", opts.URL)
}

func TestAuthPrecedence(t *testing.T) {
	// TLS takes precedence over Token
	cfg := &Config{
		Authentication: Authentication{
			TLS:   configoptional.Some(TLS{CertFile: "cert.pem", KeyFile: "key.pem"}),
			Token: configoptional.Some(Token{Token: "token-value"}),
		},
	}
	assert.NotNil(t, cfg.auth())

	// Token takes precedence over OAuth2
	cfg = &Config{
		Authentication: Authentication{
			Token:  configoptional.Some(Token{Token: "token-value"}),
			OAuth2: configoptional.Some(OAuth2{ClientID: "client-id"}),
		},
	}
	assert.NotNil(t, cfg.auth())

	// OAuth2 takes precedence over Athenz
	cfg = &Config{
		Authentication: Authentication{
			OAuth2: configoptional.Some(OAuth2{IssuerURL: "https://auth.example.com", ClientID: "client-id"}),
			Athenz: configoptional.Some(Athenz{ProviderDomain: "example.com"}),
		},
	}
	assert.NotNil(t, cfg.auth())
}
