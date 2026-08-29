// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build freebsd

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
)

type Config struct {
	Endpoint                   string         `mapstructure:"endpoint"`
	Topic                      string         `mapstructure:"topic"`
	Subscription               string         `mapstructure:"subscription"`
	Encoding                   string         `mapstructure:"encoding"`
	ConsumerName               string         `mapstructure:"consumer_name"`
	TLSTrustCertsFilePath      string         `mapstructure:"tls_trust_certs_file_path"`
	TLSAllowInsecureConnection bool           `mapstructure:"tls_allow_insecure_connection"`
	Authentication             Authentication `mapstructure:"auth"`
}

type Authentication struct {
	TLS    configoptional.Optional[TLS]    `mapstructure:"tls"`
	Token  configoptional.Optional[Token]  `mapstructure:"token"`
	Athenz configoptional.Optional[Athenz] `mapstructure:"athenz"`
	OAuth2 configoptional.Optional[OAuth2] `mapstructure:"oauth2"`
	_      struct{}
}

type TLS struct {
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	_        struct{}
}

type Token struct {
	Token configopaque.String `mapstructure:"token"`
	_     struct{}
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

func (*Config) Validate() error {
	return nil
}
