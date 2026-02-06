// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package proxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"

import (
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

// Config is the configuration for the local TCP proxy server.
type Config struct {
	// endpoint is the TCP address and port on which this receiver listens for
	// calls from the X-Ray SDK and relays them to the AWS X-Ray backend to
	// get sampling rules and report sampling statistics.
	confignet.TCPAddrConfig `mapstructure:",squash"`

	// ProxyAddress defines the proxy address that the local TCP server
	// forwards HTTP requests to AWS X-Ray backend through.
	ProxyAddress string `mapstructure:"proxy_address"`

	// TLS struct exposes TLS client configuration when forwarding
	// calls to the AWS X-Ray backend.
	TLS configtls.ClientConfig `mapstructure:"tls,omitempty"`

	// Region is the AWS region the local TCP server forwards requests to.
	Region string `mapstructure:"region"`

	// RoleARN is the IAM role used by the local TCP server when
	// communicating with the AWS X-Ray service.
	RoleARN string `mapstructure:"role_arn"`

	// AWSEndpoint is the X-Ray service endpoint which the local
	// TCP server forwards requests to.
	AWSEndpoint string `mapstructure:"aws_endpoint"`

	// LocalMode determines whether the EC2 instance metadata endpoint
	// will be called or not. Set to `true` to skip EC2 instance
	// metadata check.
	LocalMode bool `mapstructure:"local_mode"`

	// ServiceName determines which service the requests are sent to.
	// will be default to `xray`. This is mandatory for SigV4
	ServiceName string `mapstructure:"service_name"`

	// AdditionalRoutingRules defines optional routing rules for multi-service support.
	// Each route can specify its own service name, region, role, and endpoint.
	AdditionalRoutingRules []RoutingRule `mapstructure:"additional_routing_rules,omitempty"`
}

// RoutingRule defines routing configuration for a specific service.
type RoutingRule struct {
	// Paths is a list of URL paths to match against the request.
	Paths []string `mapstructure:"paths"`

	// ServiceName is the AWS service name. Required.
	ServiceName string `mapstructure:"service_name"`

	// Region is the AWS region for this service. Falls back to top-level region if not set.
	Region string `mapstructure:"region"`

	// RoleARN is the IAM role for this service. Falls back to top-level role_arn if not set.
	RoleARN string `mapstructure:"role_arn"`

	// AWSEndpoint is the AWS endpoint for this service. Auto-resolves if not set.
	AWSEndpoint string `mapstructure:"aws_endpoint"`
}

func DefaultConfig() *Config {
	return &Config{
		TCPAddrConfig: confignet.TCPAddrConfig{
			Endpoint: testutil.EndpointForPort(2000),
		},
		ProxyAddress: "",
		TLS: configtls.ClientConfig{
			Insecure:   false,
			ServerName: "",
		},
		Region:      "",
		RoleARN:     "",
		AWSEndpoint: "",
		ServiceName: "xray",
	}
}

func (cfg *Config) toSessionConfig() *sessionConfig {
	return &sessionConfig{
		Region:   cfg.Region,
		RoleARN:  cfg.RoleARN,
		Endpoint: cfg.AWSEndpoint,
	}
}

type sessionConfig struct {
	Region   string
	RoleARN  string
	Endpoint string
}
