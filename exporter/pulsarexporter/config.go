// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"

import (
	"fmt"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Pulsar exporter.
type Config struct {
	TimeoutSettings           exporterhelper.TimeoutConfig    `mapstructure:",squash"`
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`

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
	MaxConnectionsPerBroker    int            `mapstructure:"max_connections_per_broker"`
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

// Producer defines configuration for producer
type Producer struct {
	MaxReconnectToBroker            *uint            `mapstructure:"max_reconnect_broker"`
	HashingScheme                   HashingScheme    `mapstructure:"hashing_scheme"`
	CompressionLevel                CompressionLevel `mapstructure:"compression_level"`
	CompressionType                 CompressionType  `mapstructure:"compression_type"`
	MaxPendingMessages              int              `mapstructure:"max_pending_messages"`
	BatcherBuilderType              BatchBuilderType `mapstructure:"batch_builder_type"`
	PartitionsAutoDiscoveryInterval time.Duration    `mapstructure:"partitions_auto_discovery_interval"`
	BatchingMaxPublishDelay         time.Duration    `mapstructure:"batching_max_publish_delay"`
	BatchingMaxMessages             uint             `mapstructure:"batching_max_messages"`
	BatchingMaxSize                 uint             `mapstructure:"batching_max_size"`
	DisableBlockIfQueueFull         bool             `mapstructure:"disable_block_if_queue_full"`
	DisableBatching                 bool             `mapstructure:"disable_batching"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (*Config) Validate() error {
	return nil
}

func (cfg *Config) auth() pulsar.Authentication {
	authentication := cfg.Authentication
	if authentication.TLS.HasValue() {
		tlsCfg := authentication.TLS.Get()
		return pulsar.NewAuthenticationTLS(tlsCfg.CertFile, tlsCfg.KeyFile)
	}
	if authentication.Token.HasValue() {
		tokenCfg := authentication.Token.Get()
		return pulsar.NewAuthenticationToken(string(tokenCfg.Token))
	}
	if authentication.OAuth2.HasValue() {
		oauth2Cfg := authentication.OAuth2.Get()
		return pulsar.NewAuthenticationOAuth2(map[string]string{
			"type":       "client_credentials",
			"issuerUrl":  oauth2Cfg.IssuerURL,
			"clientId":   oauth2Cfg.ClientID,
			"audience":   oauth2Cfg.Audience,
			"scope":      oauth2Cfg.Scope,
			"privateKey": oauth2Cfg.PrivateKey,
		})
	}
	if authentication.Athenz.HasValue() {
		athenzCfg := authentication.Athenz.Get()
		return pulsar.NewAuthenticationAthenz(map[string]string{
			"providerDomain":  athenzCfg.ProviderDomain,
			"tenantDomain":    athenzCfg.TenantDomain,
			"tenantService":   athenzCfg.TenantService,
			"privateKey":      string(athenzCfg.PrivateKey),
			"keyId":           athenzCfg.KeyID,
			"principalHeader": athenzCfg.PrincipalHeader,
			"ztsUrl":          athenzCfg.ZtsURL,
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
	if cfg.TLSTrustCertsFilePath != "" {
		options.TLSTrustCertsFilePath = cfg.TLSTrustCertsFilePath
	}

	options.Authentication = cfg.auth()

	return options
}

func (cfg *Config) getProducerOptions() pulsar.ProducerOptions {
	producerOptions := pulsar.ProducerOptions{
		Topic:                           cfg.Topic,
		SendTimeout:                     cfg.TimeoutSettings.Timeout,
		BatcherBuilderType:              cfg.Producer.BatcherBuilderType.ToPulsar(),
		BatchingMaxMessages:             cfg.Producer.BatchingMaxMessages,
		BatchingMaxPublishDelay:         cfg.Producer.BatchingMaxPublishDelay,
		BatchingMaxSize:                 cfg.Producer.BatchingMaxSize,
		CompressionLevel:                cfg.Producer.CompressionLevel.ToPulsar(),
		CompressionType:                 cfg.Producer.CompressionType.ToPulsar(),
		DisableBatching:                 cfg.Producer.DisableBatching,
		DisableBlockIfQueueFull:         cfg.Producer.DisableBlockIfQueueFull,
		HashingScheme:                   cfg.Producer.HashingScheme.ToPulsar(),
		MaxPendingMessages:              cfg.Producer.MaxPendingMessages,
		MaxReconnectToBroker:            cfg.Producer.MaxReconnectToBroker,
		PartitionsAutoDiscoveryInterval: cfg.Producer.PartitionsAutoDiscoveryInterval,
	}
	return producerOptions
}

type BatchBuilderType string

const (
	DefaultBatchBuilder  BatchBuilderType = "default"
	KeyBasedBatchBuilder BatchBuilderType = "key_based"
)

func (c *BatchBuilderType) UnmarshalText(text []byte) error {
	switch read := BatchBuilderType(text); read {
	case DefaultBatchBuilder, KeyBasedBatchBuilder:
		*c = read
		return nil
	default:
		return fmt.Errorf("producer.compressionType should be one of 'none', 'lz4', 'zlib', or 'zstd'. configured value %v", string(read))
	}
}

func (c *BatchBuilderType) ToPulsar() pulsar.BatcherBuilderType {
	switch *c {
	case DefaultBatchBuilder:
		return pulsar.DefaultBatchBuilder
	case KeyBasedBatchBuilder:
		return pulsar.KeyBasedBatchBuilder
	default:
		return pulsar.DefaultBatchBuilder
	}
}

type CompressionType string

const (
	None CompressionType = "none"
	LZ4  CompressionType = "lz4"
	ZLib CompressionType = "zlib"
	ZStd CompressionType = "zstd"
)

func (c *CompressionType) UnmarshalText(text []byte) error {
	switch read := CompressionType(text); read {
	case None, LZ4, ZLib, ZStd:
		*c = read
		return nil
	default:
		return fmt.Errorf("producer.compressionType should be one of 'none', 'lz4', 'zlib', or 'zstd'. configured value %v", string(read))
	}
}

func (c *CompressionType) ToPulsar() pulsar.CompressionType {
	switch *c {
	case None:
		return pulsar.NoCompression
	case LZ4:
		return pulsar.LZ4
	case ZLib:
		return pulsar.ZLib
	case ZStd:
		return pulsar.ZSTD
	default:
		return pulsar.NoCompression
	}
}

type CompressionLevel string

const (
	Default CompressionLevel = "default"
	Faster  CompressionLevel = "faster"
	Better  CompressionLevel = "better"
)

func (c *CompressionLevel) UnmarshalText(text []byte) error {
	switch read := CompressionLevel(text); read {
	case Default, Faster, Better:
		*c = read
		return nil
	default:
		return fmt.Errorf("producer.compressionLevel should be one of 'default', 'faster', or 'better'. configured value %v", read)
	}
}

func (c *CompressionLevel) ToPulsar() pulsar.CompressionLevel {
	switch *c {
	case Default:
		return pulsar.Default
	case Faster:
		return pulsar.Faster
	case Better:
		return pulsar.Better
	default:
		return pulsar.Default
	}
}

type HashingScheme string

const (
	JavaStringHash HashingScheme = "java_string_hash"
	Murmur3_32Hash HashingScheme = "murmur3_32hash"
)

func (c *HashingScheme) UnmarshalText(text []byte) error {
	switch read := HashingScheme(text); read {
	case JavaStringHash, Murmur3_32Hash:
		*c = read
		return nil
	default:
		return fmt.Errorf("producer.hashingScheme should be one of 'java_string_hash' or 'murmur3_32hash'. configured value %v", read)
	}
}

func (c *HashingScheme) ToPulsar() pulsar.HashingScheme {
	switch *c {
	case JavaStringHash:
		return pulsar.JavaStringHash
	case Murmur3_32Hash:
		return pulsar.Murmur3_32Hash
	default:
		return pulsar.JavaStringHash
	}
}
