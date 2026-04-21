// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/servicediscovery/types"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
)

type routingKey int

const (
	traceIDRouting routingKey = iota
	svcRouting
	metricNameRouting
	resourceRouting
	streamIDRouting
	attrRouting
)

const (
	svcRoutingStr        = "service"
	traceIDRoutingStr    = "traceID"
	metricNameRoutingStr = "metric"
	resourceRoutingStr   = "resource"
	streamIDRoutingStr   = "streamID"
	attrRoutingStr       = "attributes"
)

// Config defines configuration for the exporter.
type Config struct {
	TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	QueueSettings             configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	Protocol Protocol         `mapstructure:"protocol"`
	Resolver ResolverSettings `mapstructure:"resolver"`

	// RoutingKey is a single routing key value
	RoutingKey string `mapstructure:"routing_key"`

	// RoutingAttributes creates a composite routing key from the listed attributes.
	//
	// For traces, attributes can come from resource, scope, or span, plus the pseudo attributes "span.kind" and
	// "span.name".
	// For metrics, attributes can come from resource, scope, or datapoint attributes.
	RoutingAttributes []string `mapstructure:"routing_attributes"`
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

	return nil
}

// Protocol holds the individual protocol-specific settings. Only OTLP is supported at the moment.
type Protocol struct {
	OTLP otlpexporter.Config `mapstructure:"otlp"`
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
}
