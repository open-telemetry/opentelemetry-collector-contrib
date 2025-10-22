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
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
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
	configgrpc.ClientConfig `mapstructure:",squash"`

	// The following fields are only used when protocol is "http"
	ProxyURL string        `mapstructure:"proxy_url,omitempty"` // Used only if protocol is http
	Timeout  time.Duration `mapstructure:"timeout,omitempty"`   // Used only if protocol is http
}

func (c *TransportConfig) ToHTTPClient(ctx context.Context, host component.Host, settings component.TelemetrySettings) (*http.Client, error) {
	headers := c.Headers
	if headers == nil {
		headers = make(map[string]configopaque.String)
	}
	headers["Content-Type"] = "application/x-protobuf"

	httpClientConfig := confighttp.ClientConfig{
		ProxyURL:        c.ProxyURL,
		TLS:             c.TLS,
		ReadBufferSize:  c.ReadBufferSize,
		WriteBufferSize: c.WriteBufferSize,
		Timeout:         c.Timeout,
		Headers:         headers,
		Compression:     c.Compression,
	}
	return httpClientConfig.ToClient(ctx, host, settings)
}

// Config defines configuration for Coralogix exporter.
type Config struct {
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"`

	// Protocol to use for communication. Options: "grpc" (default), "http"
	Protocol string `mapstructure:"protocol"`

	// Coralogix domain
	Domain string `mapstructure:"domain"`

	// Transport settings used with Domain (supports both gRPC and HTTP)
	DomainSettings TransportConfig `mapstructure:"domain_settings"`

	// Use AWS PrivateLink for the domain
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
		isEmpty(c.Traces.Endpoint) &&
		isEmpty(c.Metrics.Endpoint) &&
		isEmpty(c.Logs.Endpoint) &&
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

func (c *Config) getDomainGrpcSettings() *configgrpc.ClientConfig {
	settings := c.DomainSettings.ClientConfig
	domain := c.Domain

	// If PrivateLink is enabled, use the private link endpoint.
	// However, if the domain already contains "private", don't add it again.
	if c.PrivateLink && !strings.Contains(domain, "private.") {
		settings.Endpoint = fmt.Sprintf("ingress.private.%s:443", domain)
	} else {
		settings.Endpoint = fmt.Sprintf("ingress.%s:443", domain)
	}

	return &settings
}

// getMergedTransportConfig returns a TransportConfig that merges signal-specific settings with domain settings.
// Signal-specific settings take precedence over domain settings.
func (c *Config) getMergedTransportConfig(signalConfig *TransportConfig) *TransportConfig {
	merged := c.DomainSettings

	if signalConfig.ProxyURL != "" {
		merged.ProxyURL = signalConfig.ProxyURL
	}
	if signalConfig.Timeout != 0 {
		merged.Timeout = signalConfig.Timeout
	}

	if signalConfig.Compression != "" {
		merged.Compression = signalConfig.Compression
	}
	if signalConfig.TLS.Insecure || signalConfig.TLS.InsecureSkipVerify || signalConfig.TLS.CAFile != "" {
		merged.TLS = signalConfig.TLS
	}
	if len(signalConfig.Headers) > 0 {
		// Deep-copy domain headers to avoid mutating the shared map
		headers := make(map[string]configopaque.String)
		for k, v := range merged.Headers {
			headers[k] = v
		}
		for k, v := range signalConfig.Headers {
			headers[k] = v
		}
		merged.Headers = headers
	}
	if signalConfig.WriteBufferSize > 0 {
		merged.WriteBufferSize = signalConfig.WriteBufferSize
	}
	if signalConfig.ReadBufferSize > 0 {
		merged.ReadBufferSize = signalConfig.ReadBufferSize
	}
	if signalConfig.WaitForReady {
		merged.WaitForReady = signalConfig.WaitForReady
	}
	if signalConfig.BalancerName != "" {
		merged.BalancerName = signalConfig.BalancerName
	}
	if signalConfig.Keepalive.HasValue() {
		merged.Keepalive = signalConfig.Keepalive
	}
	if signalConfig.Auth.HasValue() {
		merged.Auth = signalConfig.Auth
	}

	if isEmpty(signalConfig.Endpoint) {
		merged.Endpoint = c.getDomainGrpcSettings().Endpoint
	} else {
		merged.Endpoint = signalConfig.Endpoint
	}

	return &merged
}
