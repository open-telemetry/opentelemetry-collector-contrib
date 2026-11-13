// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix && !solaris

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"path/filepath"
	"testing"

	"github.com/apache/pulsar-client-go/pulsar"
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
		name         string
		cfg          *Config
		expectedErr  error
		expectedErrs []error
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
			name: "multiple auth tls and token",
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
			expectedErr: errMultipleAuthMechanisms,
		},
		{
			name: "multiple auth token and oauth2",
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
			expectedErr: errMultipleAuthMechanisms,
		},
		{
			name: "multiple auth oauth2 and athenz",
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
			expectedErr: errMultipleAuthMechanisms,
		},
		{
			name: "multiple auth tls and athenz",
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
			expectedErr: errMultipleAuthMechanisms,
		},
		{
			name: "multiple auth all",
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
			expectedErr: errMultipleAuthMechanisms,
		},
		{
			name: "multiple errors returned together",
			cfg: &Config{
				Endpoint:     "pulsar://localhost:6650",
				Topic:        "test-topic",
				Subscription: "",
				Encoding:     "invalid_encoding",
				Authentication: Authentication{
					TLS:   configoptional.Some(TLS{CertFile: "cert.pem", KeyFile: "key.pem"}),
					Token: configoptional.Some(Token{Token: "token-value"}),
				},
			},
			expectedErrs: []error{errMissingSubscription, errUnsupportedEncoding, errMultipleAuthMechanisms},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			switch {
			case len(tt.expectedErrs) > 0:
				require.Error(t, err)
				for _, exp := range tt.expectedErrs {
					require.ErrorIs(t, err, exp)
				}
			case tt.expectedErr != nil:
				require.ErrorIs(t, err, tt.expectedErr)
			default:
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

func TestAuth(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected any
	}{
		{
			name: "no auth",
			cfg:  &Config{},
		},
		{
			name: "tls auth",
			cfg: &Config{
				Authentication: Authentication{
					TLS: configoptional.Some(TLS{CertFile: "cert.pem", KeyFile: "key.pem"}),
				},
			},
			expected: pulsar.NewAuthenticationTLS("cert.pem", "key.pem"),
		},
		{
			name: "token auth",
			cfg: &Config{
				Authentication: Authentication{
					Token: configoptional.Some(Token{Token: "token-value"}),
				},
			},
			expected: pulsar.NewAuthenticationToken("token-value"),
		},
		{
			name: "oauth2 auth",
			cfg: &Config{
				Authentication: Authentication{
					OAuth2: configoptional.Some(OAuth2{
						IssuerURL: "https://auth.example.com",
						ClientID:  "client-id",
					}),
				},
			},
			expected: pulsar.NewAuthenticationOAuth2(map[string]string{
				"type":      "client_credentials",
				"issuerUrl": "https://auth.example.com",
				"clientId":  "client-id",
			}),
		},
		{
			name: "athenz auth",
			cfg: &Config{
				Authentication: Authentication{
					Athenz: configoptional.Some(Athenz{ProviderDomain: "example.com"}),
				},
			},
			expected: pulsar.NewAuthenticationAthenz(map[string]string{
				"providerDomain": "example.com",
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.auth()
			if tt.expected == nil {
				assert.Nil(t, got)
			} else {
				assert.IsType(t, tt.expected, got)
			}
		})
	}
}

func TestAuthPrecedence(t *testing.T) {
	tlsAuth := configoptional.Some(TLS{CertFile: "cert.pem", KeyFile: "key.pem"})
	tokenAuth := configoptional.Some(Token{Token: "token-value"})
	oauth2Auth := configoptional.Some(OAuth2{IssuerURL: "https://auth.example.com", ClientID: "client-id"})
	athenzAuth := configoptional.Some(Athenz{ProviderDomain: "example.com"})

	expectedTLS := pulsar.NewAuthenticationTLS("cert.pem", "key.pem")
	expectedToken := pulsar.NewAuthenticationToken("token-value")
	expectedOAuth2 := pulsar.NewAuthenticationOAuth2(map[string]string{
		"type":      "client_credentials",
		"issuerUrl": "https://auth.example.com",
		"clientId":  "client-id",
	})

	tests := []struct {
		name     string
		cfg      *Config
		expected any
	}{
		// Pairwise combinations
		{
			name: "tls takes precedence over token",
			cfg: &Config{
				Authentication: Authentication{
					TLS:   tlsAuth,
					Token: tokenAuth,
				},
			},
			expected: expectedTLS,
		},
		{
			name: "tls takes precedence over oauth2",
			cfg: &Config{
				Authentication: Authentication{
					TLS:    tlsAuth,
					OAuth2: oauth2Auth,
				},
			},
			expected: expectedTLS,
		},
		{
			name: "tls takes precedence over athenz",
			cfg: &Config{
				Authentication: Authentication{
					TLS:    tlsAuth,
					Athenz: athenzAuth,
				},
			},
			expected: expectedTLS,
		},
		{
			name: "token takes precedence over oauth2",
			cfg: &Config{
				Authentication: Authentication{
					Token:  tokenAuth,
					OAuth2: oauth2Auth,
				},
			},
			expected: expectedToken,
		},
		{
			name: "token takes precedence over athenz",
			cfg: &Config{
				Authentication: Authentication{
					Token:  tokenAuth,
					Athenz: athenzAuth,
				},
			},
			expected: expectedToken,
		},
		{
			name: "oauth2 takes precedence over athenz",
			cfg: &Config{
				Authentication: Authentication{
					OAuth2: oauth2Auth,
					Athenz: athenzAuth,
				},
			},
			expected: expectedOAuth2,
		},
		// 3-way combinations
		{
			name: "tls takes precedence over token and oauth2",
			cfg: &Config{
				Authentication: Authentication{
					TLS:    tlsAuth,
					Token:  tokenAuth,
					OAuth2: oauth2Auth,
				},
			},
			expected: expectedTLS,
		},
		{
			name: "tls takes precedence over token and athenz",
			cfg: &Config{
				Authentication: Authentication{
					TLS:    tlsAuth,
					Token:  tokenAuth,
					Athenz: athenzAuth,
				},
			},
			expected: expectedTLS,
		},
		{
			name: "tls takes precedence over oauth2 and athenz",
			cfg: &Config{
				Authentication: Authentication{
					TLS:    tlsAuth,
					OAuth2: oauth2Auth,
					Athenz: athenzAuth,
				},
			},
			expected: expectedTLS,
		},
		{
			name: "token takes precedence over oauth2 and athenz",
			cfg: &Config{
				Authentication: Authentication{
					Token:  tokenAuth,
					OAuth2: oauth2Auth,
					Athenz: athenzAuth,
				},
			},
			expected: expectedToken,
		},
		// All 4
		{
			name: "tls takes precedence over all other auth mechanisms",
			cfg: &Config{
				Authentication: Authentication{
					TLS:    tlsAuth,
					Token:  tokenAuth,
					OAuth2: oauth2Auth,
					Athenz: athenzAuth,
				},
			},
			expected: expectedTLS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.IsType(t, tt.expected, tt.cfg.auth())
		})
	}
}
