// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"errors"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
)

type Config struct {
	// Configure the service URL for the Pulsar service.
	Endpoint string `mapstructure:"endpoint"`
	// The topic of pulsar to consume logs,metrics,traces. (default = "otlp_traces" for traces,
	// "otlp_metrics" for metrics, "otlp_logs" for logs)
	Topic string `mapstructure:"topic"`
	// The Subscription that receiver will be consuming messages from (default "otlp_subscription")
	Subscription string `mapstructure:"subscription"`
	// Encoding of the messages (default "otlp_proto")
	Encoding string `mapstructure:"encoding"`
	// Name specifies the consumer name.
	ConsumerName string `mapstructure:"consumer_name"`
	// Set the path to the trusted TLS certificate file
	TLSTrustCertsFilePath string `mapstructure:"tls_trust_certs_file_path"`
	// Configure whether the Pulsar client accept untrusted TLS certificate from broker (default: false)
	TLSAllowInsecureConnection bool           `mapstructure:"tls_allow_insecure_connection"`
	Authentication             Authentication `mapstructure:"auth"`
}

type Authentication struct {
	TLS    configoptional.Optional[TLS]    `mapstructure:"tls"`
	Token  configoptional.Optional[Token]  `mapstructure:"token"`
	Athenz configoptional.Optional[Athenz] `mapstructure:"athenz"`
	OAuth2 configoptional.Optional[OAuth2] `mapstructure:"oauth2"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type TLS struct {
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type Token struct {
	Token configopaque.String `mapstructure:"token"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type Athenz struct {
	ProviderDomain  string              `mapstructure:"provider_domain"`
	TenantDomain    string              `mapstructure:"tenant_domain"`
	TenantService   string              `mapstructure:"tenant_service"`
	PrivateKey      configopaque.String `mapstructure:"private_key"`
	KeyID           string              `mapstructure:"key_id"`
	PrincipalHeader string              `mapstructure:"principal_header"`
	ZtsURL          string              `mapstructure:"zts_url"`
}

type OAuth2 struct {
	IssuerURL  string `mapstructure:"issuer_url"`
	ClientID   string `mapstructure:"client_id"`
	Audience   string `mapstructure:"audience"`
	PrivateKey string `mapstructure:"private_key"`
	Scope      string `mapstructure:"scope"`
}

var _ component.Config = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (*Config) Validate() error {
	return nil
}

func (cfg *Config) auth() pulsar.Authentication {
	authentication := cfg.Authentication
	if authentication.TLS.HasValue() {
		tls := authentication.TLS.Get()
		return pulsar.NewAuthenticationTLS(tls.CertFile, tls.KeyFile)
	}
	if authentication.Token.HasValue() {
		token := authentication.Token.Get()
		return pulsar.NewAuthenticationToken(string(token.Token))
	}
	if authentication.OAuth2.HasValue() {
		oauth2 := authentication.OAuth2.Get()
		return pulsar.NewAuthenticationOAuth2(map[string]string{
			"type":       "client_credentials",
			"issuerUrl":  oauth2.IssuerURL,
			"clientId":   oauth2.ClientID,
			"audience":   oauth2.Audience,
			"scope":      oauth2.Scope,
			"privateKey": oauth2.PrivateKey,
		})
	}
	if authentication.Athenz.HasValue() {
		athenz := authentication.Athenz.Get()
		return pulsar.NewAuthenticationAthenz(map[string]string{
			"providerDomain":  athenz.ProviderDomain,
			"tenantDomain":    athenz.TenantDomain,
			"tenantService":   athenz.TenantService,
			"privateKey":      string(athenz.PrivateKey),
			"keyId":           athenz.KeyID,
			"principalHeader": athenz.PrincipalHeader,
			"ztsUrl":          athenz.ZtsURL,
		})
	}

	return nil
}

func (cfg *Config) clientOptions() pulsar.ClientOptions {
	url := cfg.Endpoint
	if url == "" {
		url = defaultServiceURL
	}
	options := pulsar.ClientOptions{
		URL: url,
	}

	options.TLSAllowInsecureConnection = cfg.TLSAllowInsecureConnection
	if cfg.TLSTrustCertsFilePath != "" {
		options.TLSTrustCertsFilePath = cfg.TLSTrustCertsFilePath
	}

	auth := cfg.auth()
	options.Authentication = auth
	return options
}

func (cfg *Config) consumerOptions() (pulsar.ConsumerOptions, error) {
	options := pulsar.ConsumerOptions{
		Type:             pulsar.Failover,
		Topic:            cfg.Topic,
		SubscriptionName: cfg.Subscription,
	}

	if cfg.ConsumerName != "" {
		options.Name = cfg.ConsumerName
	}

	if options.SubscriptionName == "" || options.Topic == "" {
		return options, errors.New("topic and subscription is required")
	}

	return options, nil
}
