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

package pulsarexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"

import (
	"fmt"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Pulsar exporter.
type Config struct {
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	// Endpoint of pulsar broker (default "pulsar://localhost:6650")
	Endpoint string `mapstructure:"endpoint"`
	// The name of the pulsar topic to export to (default otlp_spans for traces, otlp_metrics for metrics)
	Topic string `mapstructure:"topic"`
	// Encoding of messages (default "otlp_proto")
	Encoding string `mapstructure:"encoding"`
	// Producer configuration of the Pulsar producer
	Producer Producer `mapstructure:"producer"`
	// Set the path to the trusted TLS certificate file
	TLSTrustCertsFilePath string `mapstructure:"tls_trust_certs_file_path"`
	// Configure whether the Pulsar client accept untrusted TLS certificate from broker (default: false)
	TLSAllowInsecureConnection bool           `mapstructure:"tls_allow_insecure_connection"`
	Authentication             Authentication `mapstructure:"auth"`
	OperationTimeout           time.Duration  `mapstructure:"operation_timeout"`
	ConnectionTimeout          time.Duration  `mapstructure:"connection_timeout"`
	MaxConnectionsPerBroker    int            `mapstructure:"map_connections_per_broker"`
}

type Authentication struct {
	TLS    *TLS    `mapstructure:"tls"`
	Token  *Token  `mapstructure:"token"`
	Athenz *Athenz `mapstructure:"athenz"`
	OAuth2 *OAuth2 `mapstructure:"oauth2"`
}

type TLS struct {
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

type Token struct {
	Token configopaque.String `mapstructure:"token"`
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
	IssuerURL string `mapstructure:"issuer_url"`
	ClientID  string `mapstructure:"client_id"`
	Audience  string `mapstructure:"audience"`
}

// Producer defines configuration for producer
type Producer struct {
	MaxReconnectToBroker            *uint         `mapstructure:"max_reconnect_broker"`
	HashingScheme                   string        `mapstructure:"hashing_scheme"`
	CompressionLevel                string        `mapstructure:"compression_level"`
	CompressionType                 string        `mapstructure:"compression_type"`
	MaxPendingMessages              int           `mapstructure:"max_pending_messages"`
	BatcherBuilderType              string        `mapstructure:"batch_builder_type"`
	PartitionsAutoDiscoveryInterval time.Duration `mapstructure:"partitions_auto_discovery_interval"`
	BatchingMaxPublishDelay         time.Duration `mapstructure:"batching_max_publish_delay"`
	BatchingMaxMessages             uint          `mapstructure:"batching_max_messages"`
	BatchingMaxSize                 uint          `mapstructure:"batching_max_size"`
	DisableBlockIfQueueFull         bool          `mapstructure:"disable_block_if_queue_full"`
	DisableBatching                 bool          `mapstructure:"disable_batching"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {

	return nil
}

func (cfg *Config) auth() pulsar.Authentication {
	authentication := cfg.Authentication
	if authentication.TLS != nil {
		return pulsar.NewAuthenticationTLS(authentication.TLS.CertFile, authentication.TLS.KeyFile)
	}
	if authentication.Token != nil {
		return pulsar.NewAuthenticationToken(string(authentication.Token.Token))
	}
	if authentication.OAuth2 != nil {
		return pulsar.NewAuthenticationOAuth2(map[string]string{
			"issuerUrl": authentication.OAuth2.IssuerURL,
			"clientId":  authentication.OAuth2.ClientID,
			"audience":  authentication.OAuth2.Audience,
		})
	}
	if authentication.Athenz != nil {
		return pulsar.NewAuthenticationAthenz(map[string]string{
			"providerDomain":  authentication.Athenz.ProviderDomain,
			"tenantDomain":    authentication.Athenz.TenantDomain,
			"tenantService":   authentication.Athenz.TenantService,
			"privateKey":      string(authentication.Athenz.PrivateKey),
			"keyId":           authentication.Athenz.KeyID,
			"principalHeader": authentication.Athenz.PrincipalHeader,
			"ztsUrl":          authentication.Athenz.ZtsURL,
		})
	}

	return nil
}

func (cfg *Config) clientOptions() pulsar.ClientOptions {
	options := pulsar.ClientOptions{
		URL:                     cfg.Endpoint,
		ConnectionTimeout:       cfg.ConnectionTimeout,
		OperationTimeout:        cfg.OperationTimeout,
		MaxConnectionsPerBroker: cfg.MaxConnectionsPerBroker,
	}

	options.TLSAllowInsecureConnection = cfg.TLSAllowInsecureConnection
	if len(cfg.TLSTrustCertsFilePath) > 0 {
		options.TLSTrustCertsFilePath = cfg.TLSTrustCertsFilePath
	}

	options.Authentication = cfg.auth()

	return options
}

func (cfg *Config) getProducerOptions() (pulsar.ProducerOptions, error) {

	producerOptions := pulsar.ProducerOptions{
		Topic:                           cfg.Topic,
		SendTimeout:                     cfg.Timeout,
		DisableBatching:                 cfg.Producer.DisableBatching,
		DisableBlockIfQueueFull:         cfg.Producer.DisableBlockIfQueueFull,
		MaxPendingMessages:              cfg.Producer.MaxPendingMessages,
		BatchingMaxPublishDelay:         cfg.Producer.BatchingMaxPublishDelay,
		BatchingMaxSize:                 cfg.Producer.BatchingMaxSize,
		BatchingMaxMessages:             cfg.Producer.BatchingMaxMessages,
		PartitionsAutoDiscoveryInterval: cfg.Producer.PartitionsAutoDiscoveryInterval,
		MaxReconnectToBroker:            cfg.Producer.MaxReconnectToBroker,
	}

	batchBuilderType, err := stringToBatchBuilderType(cfg.Producer.BatcherBuilderType)
	if err != nil {
		return producerOptions, err
	}
	producerOptions.BatcherBuilderType = batchBuilderType

	compressionType, err := stringToCompressionType(cfg.Producer.CompressionType)
	if err != nil {
		return producerOptions, err
	}
	producerOptions.CompressionType = compressionType

	compressionLevel, err := stringToCompressionLevel(cfg.Producer.CompressionLevel)
	if err != nil {
		return producerOptions, err
	}
	producerOptions.CompressionLevel = compressionLevel

	hashingScheme, err := stringToHashingScheme(cfg.Producer.HashingScheme)
	if err != nil {
		return producerOptions, err
	}
	producerOptions.HashingScheme = hashingScheme

	return producerOptions, nil
}

func stringToBatchBuilderType(builderType string) (pulsar.BatcherBuilderType, error) {
	switch builderType {
	case "default":
		return pulsar.DefaultBatchBuilder, nil
	case "key_based":
		return pulsar.KeyBasedBatchBuilder, nil
	default:
		return pulsar.DefaultBatchBuilder, fmt.Errorf("producer.batchBuilderType should be one of 'default' or 'key_based'. configured value %v. Assigning default value as default", builderType)
	}
}

func stringToCompressionType(compressionType string) (pulsar.CompressionType, error) {
	switch compressionType {
	case "none":
		return pulsar.NoCompression, nil
	case "lz4":
		return pulsar.LZ4, nil
	case "zlib":
		return pulsar.ZLib, nil
	case "zstd":
		return pulsar.ZSTD, nil
	default:
		return pulsar.NoCompression, fmt.Errorf("producer.compressionType should be one of 'none', 'lz4', 'zlib', or 'zstd'. configured value %v. Assigning default value as none", compressionType)
	}
}

func stringToCompressionLevel(compressionLevel string) (pulsar.CompressionLevel, error) {
	switch compressionLevel {
	case "default":
		return pulsar.Default, nil
	case "faster":
		return pulsar.Faster, nil
	case "better":
		return pulsar.Better, nil
	default:
		return pulsar.Default, fmt.Errorf("producer.compressionLevel should be one of 'default', 'faster', or 'better'. configured value %v. Assigning default value as default", compressionLevel)
	}
}

func stringToHashingScheme(hashingScheme string) (pulsar.HashingScheme, error) {
	switch hashingScheme {
	case "java_string_hash":
		return pulsar.JavaStringHash, nil
	case "murmur3_32hash":
		return pulsar.Murmur3_32Hash, nil
	default:
		return pulsar.JavaStringHash, fmt.Errorf("producer.hashingScheme should be one of 'java_string_hash' or 'murmur3_32hash'. configured value %v, Assigning default value as java_string_hash", hashingScheme)
	}
}
