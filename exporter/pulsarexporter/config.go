// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulsarexporter

import (
	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Pulsar exporter.
type Config struct {
	config.ExporterSettings        `mapstructure:",squash"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	// Endpoint of pulsar broker (default "pulsar://localhost:6650")
	Endpoint string `mapstructure:"endpoint"`
	// The name of the pulsar topic to export to (default otlp_spans for traces, otlp_metrics for metrics)
	Topic string `mapstructure:"topic"`
	// Encoding of messages (default "otlp_proto")
	Encoding string `mapstructure:"encoding"`
	// Set the path to the trusted TLS certificate file
	TLSTrustCertsFilePath string `mapstructure:"tls_trust_certs_file_path"`
	// Configure whether the Pulsar client accept untrusted TLS certificate from broker (default: false)
	Insecure       bool           `mapstructure:"insecure"`
	Authentication Authentication `mapstructure:"auth"`
}

type Authentication struct {
	TLS    *TLS    `mapstructure:"tls"`
	Token  *Token  `mapstructure:"Token"`
	Athenz *Athenz `mapstructure:"athenz"`
	OAuth2 *OAuth2 `mapstructure:"oauth2"`
}

type TLS struct {
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

type Token struct {
	Token string `mapstructure:"Token"`
}

type Athenz struct {
	ProviderDomain  string `mapstructure:"provider_domain"`
	TenantDomain    string `mapstructure:"tenant_domain"`
	TenantService   string `mapstructure:"tenant_service"`
	PrivateKey      string `mapstructure:"private_key"`
	KeyId           string `mapstructure:"key_id"`
	PrincipalHeader string `mapstructure:"principal_header"`
	ZtsUrl          string `mapstructure:"zts_url"`
}

type OAuth2 struct {
	IssuerUrl string `mapstructure:"issuer_url"`
	ClientId  string `mapstructure:"client_id"`
	Audience  string `mapstructure:"audience"`
}

var _ config.Exporter = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {

	return nil
}

func (cfg *Config) auth() (pulsar.Authentication, error) {
	authentication := cfg.Authentication
	if authentication.TLS != nil {
		return pulsar.NewAuthenticationTLS(authentication.TLS.CertFile, authentication.TLS.KeyFile), nil
	}
	if authentication.Token != nil {
		return pulsar.NewAuthenticationToken(authentication.Token.Token), nil
	}
	if authentication.OAuth2 != nil {
		return pulsar.NewAuthenticationOAuth2(map[string]string{
			"issuerUrl": authentication.OAuth2.IssuerUrl,
			"clientId":  authentication.OAuth2.ClientId,
			"audience":  authentication.OAuth2.Audience,
		}), nil
	}
	if authentication.Athenz != nil {
		return pulsar.NewAuthenticationAthenz(map[string]string{
			"providerDomain":  authentication.Athenz.ProviderDomain,
			"tenantDomain":    authentication.Athenz.TenantDomain,
			"tenantService":   authentication.Athenz.TenantService,
			"privateKey":      authentication.Athenz.PrivateKey,
			"keyId":           authentication.Athenz.KeyId,
			"principalHeader": authentication.Athenz.PrincipalHeader,
			"ztsUrl":          authentication.Athenz.ZtsUrl,
		}), nil
	}

	return nil, nil
}

func (cfg *Config) clientOptions() (pulsar.ClientOptions, error) {
	options := pulsar.ClientOptions{
		URL: cfg.Endpoint,
	}

	options.TLSAllowInsecureConnection = cfg.Insecure
	if len(cfg.TLSTrustCertsFilePath) > 0 {
		options.TLSTrustCertsFilePath = cfg.TLSTrustCertsFilePath
	}

	auth, err := cfg.auth()
	if err != nil {
		return pulsar.ClientOptions{}, err
	}

	options.Authentication = auth

	return options, nil
}
