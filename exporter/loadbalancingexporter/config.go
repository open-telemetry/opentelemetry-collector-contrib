// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/servicediscovery/types"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
)

type routingKey int

const (
	traceIDRouting routingKey = iota
	svcRouting
	metricNameRouting
	resourceRouting
	streamIDRouting
	attrRouting
	randomnessRouting
)

const (
	svcRoutingStr        = "service"
	traceIDRoutingStr    = "traceID"
	metricNameRoutingStr = "metric"
	resourceRoutingStr   = "resource"
	streamIDRoutingStr   = "streamID"
	attrRoutingStr       = "attributes"
	randomnessRoutingStr = "randomness"
)

// Config defines configuration for the exporter.
type Config struct {
	TimeoutSettings exporterhelper.TimeoutConfig                             `mapstructure:",squash"`
	BackOffConfig   configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`
	QueueSettings   configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	Protocol Protocol         `mapstructure:"protocol"`
	Resolver ResolverSettings `mapstructure:"resolver"`

	// RoutingKey is a single routing key value
	RoutingKey string `mapstructure:"routing_key"`

	// RoutingAttributes creates a composite routing key from the listed attributes.
	//
	// For traces, attributes can come from resource, scope, or span, plus the pseudo attributes "span.kind" and
	// "span.name".
	// For logs, attributes can come from resource, scope, or log record attributes, plus the pseudo attributes
	// "log.severity" and "log.body".
	// For metrics, attributes can come from resource, scope, or datapoint attributes.
	// Keys are encoded as "name=value|name=value|" in the order configured. Missing attributes are encoded as "name=|".
	// Non-string values are deterministically stringified.
	RoutingAttributes []string `mapstructure:"routing_attributes"`
}

var errMultipleProtocolsProvided = errors.New("only one of protocol::otlp and protocol::otlp_http should be specified")

// Unmarshal rejects a configuration that sets both protocols. protocol::otlp is always populated by the
// default config, so only the raw configuration can tell whether the user wrote it.
func (c *Config) Unmarshal(conf *confmap.Conf) error {
	if conf.IsSet("protocol::otlp") && conf.IsSet("protocol::otlp_http") {
		return errMultipleProtocolsProvided
	}
	return conf.Unmarshal(c)
}

// Validate checks if the exporter configuration is valid.
func (c *Config) Validate() error {
	// routing_attributes only has meaning when routing_key=attributes.
	if c.RoutingKey == attrRoutingStr && len(c.RoutingAttributes) == 0 {
		return fmt.Errorf("routing_attributes must be specified when routing_key is %q", attrRoutingStr)
	}

	if c.RoutingKey != attrRoutingStr && len(c.RoutingAttributes) > 0 {
		return fmt.Errorf("routing_attributes can only be used when routing_key is %q; got %q. Remove routing_attributes or set routing_key to %q", attrRoutingStr, c.RoutingKey, attrRoutingStr)
	}

	if c.Protocol.OTLPHTTP.HasValue() {
		return validateHTTPEndpoints(c.Protocol.OTLPHTTP.Get())
	}

	return nil
}

// validateHTTPEndpoints checks that every endpoint set on the OTLP/HTTP sub-exporter has a scheme and a host.
// The host and port are replaced by the resolved backend, the rest of the URL is kept.
func validateHTTPEndpoints(cfg *otlphttpexporter.Config) error {
	for _, e := range []struct{ name, value string }{
		{"endpoint", cfg.ClientConfig.Endpoint},
		{"traces_endpoint", cfg.TracesEndpoint},
		{"metrics_endpoint", cfg.MetricsEndpoint},
		{"logs_endpoint", cfg.LogsEndpoint},
		{"profiles_endpoint", cfg.ProfilesEndpoint},
	} {
		if e.value == "" {
			continue
		}
		u, err := url.Parse(e.value)
		if err != nil {
			return fmt.Errorf("protocol::otlp_http::%s is not a valid URL: %w", e.name, err)
		}
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("protocol::otlp_http::%s must be an http:// or https:// URL with a host, got %q", e.name, e.value)
		}
	}
	return nil
}

// Protocol holds the individual protocol-specific settings. OTLP/gRPC is used unless OTLPHTTP is set.
type Protocol struct {
	OTLP otlpexporter.Config `mapstructure:"otlp"`
	// OTLPHTTP switches the backend exporters to OTLP/HTTP. It cannot be combined with OTLP.
	OTLPHTTP configoptional.Optional[otlphttpexporter.Config] `mapstructure:"otlp_http"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// ResolverSettings defines the configurations for the backend resolver
type ResolverSettings struct {
	Static      configoptional.Optional[StaticResolver]      `mapstructure:"static"`
	DNS         configoptional.Optional[DNSResolver]         `mapstructure:"dns"`
	K8sSvc      configoptional.Optional[K8sSvcResolver]      `mapstructure:"k8s"`
	AWSCloudMap configoptional.Optional[AWSCloudMapResolver] `mapstructure:"aws_cloud_map"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// StaticResolver defines the configuration for the resolver providing a fixed list of backends
type StaticResolver struct {
	Hostnames []string `mapstructure:"hostnames"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// DNSResolver defines the configuration for the DNS resolver
type DNSResolver struct {
	Hostname string        `mapstructure:"hostname"`
	Port     string        `mapstructure:"port"`
	Interval time.Duration `mapstructure:"interval"`
	Timeout  time.Duration `mapstructure:"timeout"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// K8sSvcResolver defines the configuration for the DNS resolver
type K8sSvcResolver struct {
	Service         string        `mapstructure:"service"`
	Ports           []int32       `mapstructure:"ports"`
	Timeout         time.Duration `mapstructure:"timeout"`
	ReturnHostnames bool          `mapstructure:"return_hostnames"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type AWSCloudMapResolver struct {
	NamespaceName string                   `mapstructure:"namespace"`
	ServiceName   string                   `mapstructure:"service_name"`
	HealthStatus  types.HealthStatusFilter `mapstructure:"health_status"`
	Interval      time.Duration            `mapstructure:"interval"`
	Timeout       time.Duration            `mapstructure:"timeout"`
	Port          *uint16                  `mapstructure:"port"`
	OwnerAccount  *string                  `mapstructure:"owner_account"`
}
