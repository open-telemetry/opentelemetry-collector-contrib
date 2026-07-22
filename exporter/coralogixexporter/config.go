// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/gzip"
)

const (
	cxAppNameAttrName       = "cx.application.name"
	cxSubsystemNameAttrName = "cx.subsystem.name"
	httpProtocol            = "http"
	grpcProtocol            = "grpc"
)

// TransportConfig extends configgrpc.ClientConfig with additional HTTP-specific settings
type TransportConfig struct {
	// Embed the gRPC configuration to ensure backward compatibility
	ClientConfig configgrpc.ClientConfig `mapstructure:",squash"`

	// The following fields are only used when protocol is "http"
	ProxyURL string        `mapstructure:"proxy_url,omitempty"` // Used only if protocol is http
	Timeout  time.Duration `mapstructure:"timeout,omitempty"`   // Used only if protocol is http

	// AcceptEncoding specifies the compression encoding to accept for gRPC responses.
	// Defaults to "gzip" if not set. Only used when protocol is "grpc".
	AcceptEncoding string `mapstructure:"accept_encoding,omitempty"`
}

func (c *TransportConfig) ToHTTPClient(ctx context.Context, host component.Host, settings component.TelemetrySettings) (*http.Client, error) {
	c.ClientConfig.Headers.Set("Content-Type", "application/x-protobuf")

	httpClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	httpClientConfig.MaxIdleConns = 0
	httpClientConfig.IdleConnTimeout = 0
	httpClientConfig.ForceAttemptHTTP2 = false
	httpClientConfig.ProxyURL = c.ProxyURL
	httpClientConfig.TLS = c.ClientConfig.TLS
	httpClientConfig.ReadBufferSize = c.ClientConfig.ReadBufferSize
	httpClientConfig.WriteBufferSize = c.ClientConfig.WriteBufferSize
	httpClientConfig.Timeout = c.Timeout
	httpClientConfig.Headers = c.ClientConfig.Headers
	httpClientConfig.Compression = c.ClientConfig.Compression
	return httpClientConfig.ToClient(ctx, host.GetExtensions(), settings)
}

// GetAcceptEncoding returns the accept encoding to use for gRPC responses.
func (c *TransportConfig) GetAcceptEncoding() string {
	return c.AcceptEncoding
}

// Config defines configuration for Coralogix exporter.
type Config struct {
	QueueSettings             configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"`

	// Protocol to use for communication. Options: "grpc" (default), "http"
	Protocol string `mapstructure:"protocol"`

	// Coralogix domain
	Domain string `mapstructure:"domain"`

	// Transport settings used with Domain (supports both gRPC and HTTP)
	DomainSettings TransportConfig `mapstructure:"domain_settings"`

	// PrivateLink, when enabled, automatically routes telemetry to the AWS PrivateLink ingress
	// endpoint (ingress.private.<domain>) for the configured domain. AWS PrivateLink provides
	// private connectivity between VPCs, supported AWS services, and on-premises networks
	// without exposing traffic to the public internet. If the domain already contains
	// "private.", no additional prefix is added. For up-to-date PrivateLink endpoints, see
	// https://coralogix.com/docs/integrations/aws/aws-privatelink/aws-privatelink/#privatelink-endpoints
	PrivateLink bool `mapstructure:"private_link"`

	// Coralogix traces ingress endpoint (supports both gRPC and HTTP)
	Traces TransportConfig `mapstructure:"traces"`

	// The Coralogix metrics ingress endpoint (supports both gRPC and HTTP)
	Metrics TransportConfig `mapstructure:"metrics"`

	// The Coralogix logs ingress endpoint (supports both gRPC and HTTP)
	Logs TransportConfig `mapstructure:"logs"`

	// The Coralogix profiles ingress endpoint (gRPC only)
	Profiles configgrpc.ClientConfig `mapstructure:"profiles"`

	// Your Coralogix private key (sensitive) for authentication
	PrivateKey configopaque.String `mapstructure:"private_key"`

	// Ordered list of Resource attributes that are used for Coralogix
	// AppName and SubSystem values. The first non-empty Resource attribute is used.
	// Example: AppNameAttributes: ["k8s.namespace.name", "service.namespace"]
	// Example: SubSystemAttributes: ["k8s.deployment.name", "k8s.daemonset.name", "service.name"]
	AppNameAttributes   []string `mapstructure:"application_name_attributes"`
	SubSystemAttributes []string `mapstructure:"subsystem_name_attributes"`
	// Default Coralogix application and subsystem name values.
	AppName   string `mapstructure:"application_name"`
	SubSystem string `mapstructure:"subsystem_name"`

	RateLimiter RateLimiterConfig `mapstructure:"rate_limiter"`
}

var _ confmap.Unmarshaler = (*Config)(nil)

// ensureHTTPScheme ensures the endpoint has an https:// scheme
func ensureHTTPScheme(endpoint string) string {
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return "https://" + endpoint
	}
	return endpoint
}

func (c *Config) Unmarshal(conf *confmap.Conf) error {
	if err := conf.Unmarshal(c); err != nil {
		return err
	}

	_ = setDomainGrpcSettings(c)
	var err error

	if !isEmpty(c.Domain) && isEmpty(c.Metrics.ClientConfig.Endpoint) {
		c.Metrics, err = setMergedTransportConfigWithConf(conf, c, "metrics", &c.Metrics)
		if err != nil {
			return err
		}
	}

	if isEmpty(c.Metrics.ClientConfig.Endpoint) {
		c.Metrics.ClientConfig.Endpoint = ensureHTTPScheme(c.DomainSettings.ClientConfig.Endpoint)
	} else if c.Protocol == httpProtocol {
		c.Metrics.ClientConfig.Endpoint = ensureHTTPScheme(c.Metrics.ClientConfig.Endpoint)
	}

	if !isEmpty(c.Domain) && isEmpty(c.Traces.ClientConfig.Endpoint) {
		c.Traces, err = setMergedTransportConfigWithConf(conf, c, "traces", &c.Traces)
		if err != nil {
			return err
		}
	}

	if isEmpty(c.Traces.ClientConfig.Endpoint) {
		c.Traces.ClientConfig.Endpoint = ensureHTTPScheme(c.DomainSettings.ClientConfig.Endpoint)
	} else if c.Protocol == httpProtocol {
		c.Traces.ClientConfig.Endpoint = ensureHTTPScheme(c.Traces.ClientConfig.Endpoint)
	}

	if !isEmpty(c.Domain) && isEmpty(c.Logs.ClientConfig.Endpoint) {
		c.Logs, err = setMergedTransportConfigWithConf(conf, c, "logs", &c.Logs)
		if err != nil {
			return err
		}
	}

	if isEmpty(c.Logs.ClientConfig.Endpoint) {
		c.Logs.ClientConfig.Endpoint = ensureHTTPScheme(c.DomainSettings.ClientConfig.Endpoint)
	} else if c.Protocol == httpProtocol {
		c.Logs.ClientConfig.Endpoint = ensureHTTPScheme(c.Logs.ClientConfig.Endpoint)
	}

	// Only auto-populate profiles endpoint if protocol is not HTTP (profiles only support gRPC)
	if c.Protocol != httpProtocol && !isEmpty(c.Domain) && isEmpty(c.Profiles.Endpoint) {
		tCfg, err := setMergedTransportConfigWithConf(conf, c, "profiles", &TransportConfig{ClientConfig: c.Profiles})
		if err != nil {
			return err
		}
		c.Profiles = tCfg.ClientConfig
	}

	// Only set profiles endpoint fallback if protocol is not HTTP and we have something to fallback to
	// This avoid validation issues
	if c.Protocol != httpProtocol && isEmpty(c.Profiles.Endpoint) && !isEmpty(c.DomainSettings.ClientConfig.Endpoint) {
		c.Profiles.Endpoint = ensureHTTPScheme(c.DomainSettings.ClientConfig.Endpoint)
	}
	return nil
}

type RateLimiterConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	Threshold int           `mapstructure:"threshold"`
	Duration  time.Duration `mapstructure:"duration"`
}

func isEmpty(endpoint string) bool {
	if endpoint == "" || endpoint == "https://" || endpoint == "http://" {
		return true
	}
	return false
}

func (c *Config) Validate() error {
	if c.Protocol != grpcProtocol && c.Protocol != httpProtocol && c.Protocol != "" {
		return fmt.Errorf("protocol must be %s or %s", grpcProtocol, httpProtocol)
	}

	// validate that at least one endpoint is set up correctly
	if isEmpty(c.Domain) &&
		isEmpty(c.Traces.ClientConfig.Endpoint) &&
		isEmpty(c.Metrics.ClientConfig.Endpoint) &&
		isEmpty(c.Logs.ClientConfig.Endpoint) &&
		isEmpty(c.Profiles.Endpoint) {
		return errors.New("`domain` or `traces.endpoint` or `metrics.endpoint` or `logs.endpoint` or `profiles.endpoint` not specified, please fix the configuration")
	}
	if c.PrivateKey == "" {
		return errors.New("`private_key` not specified, please fix the configuration")
	}
	if c.AppName == "" {
		return errors.New("`application_name` not specified, please fix the configuration")
	}

	if c.RateLimiter.Enabled {
		if c.RateLimiter.Threshold <= 0 {
			return errors.New("`rate_limiter.threshold` must be greater than 0")
		}
		if c.RateLimiter.Duration <= 0 {
			return errors.New("`rate_limiter.duration` must be greater than 0")
		}
	}

	// Validate that HTTP protocol is not used with profiles
	if c.Protocol == httpProtocol && !isEmpty(c.Profiles.Endpoint) {
		return errors.New("profiles signal is not supported with HTTP protocol, use gRPC protocol (default) instead")
	}

	// Validate accept_encoding for gRPC protocol
	if c.Protocol != httpProtocol {
		if err := validateAcceptEncoding(c.DomainSettings.AcceptEncoding); err != nil {
			return fmt.Errorf("domain_settings.accept_encoding: %w", err)
		}
		if err := validateAcceptEncoding(c.Traces.AcceptEncoding); err != nil {
			return fmt.Errorf("traces.accept_encoding: %w", err)
		}
		if err := validateAcceptEncoding(c.Metrics.AcceptEncoding); err != nil {
			return fmt.Errorf("metrics.accept_encoding: %w", err)
		}
		if err := validateAcceptEncoding(c.Logs.AcceptEncoding); err != nil {
			return fmt.Errorf("logs.accept_encoding: %w", err)
		}
	}

	return nil
}

// validateAcceptEncoding checks if the given encoding is supported by gRPC.
// Empty string is allowed (defaults to gzip).
func validateAcceptEncoding(encodingName string) error {
	// Empty string is valid (will default to gzip)
	if encodingName == "" {
		return nil
	}

	// Check if the compressor is registered in gRPC
	if encoding.GetCompressor(encodingName) == nil {
		return fmt.Errorf("unsupported compression encoding %q, must be a registered gRPC compressor (e.g., \"gzip\")", encodingName)
	}

	return nil
}

func (c *Config) getMetadataFromResource(res pcommon.Resource) (appName, subsystem string) {
	// Example application name attributes: service.namespace, k8s.namespace.name
	for _, appNameAttribute := range c.AppNameAttributes {
		attr, ok := res.Attributes().Get(appNameAttribute)
		if ok && attr.AsString() != "" {
			appName = attr.AsString()
			break
		}
	}

	// Example subsystem name attributes: service.name, k8s.deployment.name, k8s.statefulset.name
	for _, subSystemNameAttribute := range c.SubSystemAttributes {
		attr, ok := res.Attributes().Get(subSystemNameAttribute)
		if ok && attr.AsString() != "" {
			subsystem = attr.AsString()
			break
		}
	}

	if appName == "" {
		appName = c.AppName
	}
	if subsystem == "" {
		subsystem = c.SubSystem
	}

	if appName == "" {
		attr, ok := res.Attributes().Get(cxAppNameAttrName)
		if ok && attr.AsString() != "" {
			appName = attr.AsString()
		}
	}
	if subsystem == "" {
		attr, ok := res.Attributes().Get(cxSubsystemNameAttrName)
		if ok && attr.AsString() != "" {
			subsystem = attr.AsString()
		}
	}
	return appName, subsystem
}

func setDomainGrpcSettings(c *Config) string {
	settings := c.DomainSettings.ClientConfig
	domain := c.Domain

	// If PrivateLink is enabled, use the private link endpoint.
	// However, if the domain already contains "private", don't add it again.
	if c.PrivateLink && !strings.Contains(domain, "private.") {
		settings.Endpoint = fmt.Sprintf("ingress.private.%s:443", domain)
	} else {
		settings.Endpoint = fmt.Sprintf("ingress.%s:443", domain)
	}

	return settings.Endpoint
}

// setMergedTransportConfigWithConf returns a TransportConfig that merges signal-specific settings with domain settings.
// Signal-specific settings take precedence over domain settings.
// This is used when unmarshaling from confmap to get domain settings.
// We pass the conf to get a fresh copy of the domain settings from the confmap.
//
// NOTE: This function modifies *Config and *TransportConfig passed as arguments.
// DO NOT ADD FURTHER USES OUTSIDE OF Unmarshal, see github.com/open-telemetry/opentelemetry-collector-contrib/issues/44731
func setMergedTransportConfigWithConf(conf *confmap.Conf, c *Config, signalKey string, signalConfig *TransportConfig) (TransportConfig, error) {
	var domainSettings TransportConfig
	sub, err := conf.Sub("domain_settings")
	if err != nil {
		return TransportConfig{}, err
	}
	if unmarshalErr := sub.Unmarshal(&domainSettings); unmarshalErr != nil {
		return TransportConfig{}, unmarshalErr
	}

	var signalSub *confmap.Conf
	if signalKey != "" {
		signalSub, err = conf.Sub(signalKey)
		if err != nil {
			return TransportConfig{}, err
		}
	}

	merged := setMergedTransportConfig(c, &domainSettings, signalSub, signalConfig)
	applyTransportDefaults(merged)
	return *merged, nil
}

// setMergedTransportConfig returns a TransportConfig that merges signal-specific settings with domain settings.
// Signal-specific settings take precedence over domain settings.
//
// NOTE: This function modifies *Config and *TransportConfig passed as arguments.
// DO NOT ADD FURTHER USES OUTSIDE OF Unmarshal, see github.com/open-telemetry/opentelemetry-collector-contrib/issues/44731
func setMergedTransportConfig(c *Config, merged *TransportConfig, signalSub *confmap.Conf, signalConfig *TransportConfig) *TransportConfig {
	if signalConfig.ProxyURL != "" {
		merged.ProxyURL = signalConfig.ProxyURL
	}
	if signalConfig.Timeout != 0 {
		merged.Timeout = signalConfig.Timeout
	}

	if signalSub != nil && signalSub.IsSet("compression") {
		merged.ClientConfig.Compression = signalConfig.ClientConfig.Compression
	}
	if signalSub != nil && signalSub.IsSet("accept_encoding") {
		merged.AcceptEncoding = signalConfig.AcceptEncoding
	}
	if signalConfig.ClientConfig.TLS.Insecure || signalConfig.ClientConfig.TLS.InsecureSkipVerify || signalConfig.ClientConfig.TLS.CAFile != "" {
		merged.ClientConfig.TLS = signalConfig.ClientConfig.TLS
	}
	if len(signalConfig.ClientConfig.Headers) > 0 {
		// MapList.Set copies the backing array, so this does not mutate c.DomainSettings
		for k, v := range signalConfig.ClientConfig.Headers.Iter {
			merged.ClientConfig.Headers.Set(k, v)
		}
	}
	if signalConfig.ClientConfig.WriteBufferSize > 0 {
		merged.ClientConfig.WriteBufferSize = signalConfig.ClientConfig.WriteBufferSize
	}
	if signalConfig.ClientConfig.ReadBufferSize > 0 {
		merged.ClientConfig.ReadBufferSize = signalConfig.ClientConfig.ReadBufferSize
	}
	if signalConfig.ClientConfig.WaitForReady {
		merged.ClientConfig.WaitForReady = signalConfig.ClientConfig.WaitForReady
	}
	if signalConfig.ClientConfig.BalancerName != "" {
		merged.ClientConfig.BalancerName = signalConfig.ClientConfig.BalancerName
	}
	if signalConfig.ClientConfig.Keepalive.HasValue() {
		merged.ClientConfig.Keepalive = signalConfig.ClientConfig.Keepalive
	}
	if signalConfig.ClientConfig.Auth.HasValue() {
		merged.ClientConfig.Auth = signalConfig.ClientConfig.Auth
	}

	if isEmpty(signalConfig.ClientConfig.Endpoint) {
		merged.ClientConfig.Endpoint = setDomainGrpcSettings(c)
	} else {
		merged.ClientConfig.Endpoint = signalConfig.ClientConfig.Endpoint
	}

	return merged
}

func applyTransportDefaults(cfg *TransportConfig) {
	if cfg.ClientConfig.Compression == "" {
		cfg.ClientConfig.Compression = configcompression.TypeGzip
	}
	if cfg.AcceptEncoding == "" {
		cfg.AcceptEncoding = gzip.Name
	}
}
