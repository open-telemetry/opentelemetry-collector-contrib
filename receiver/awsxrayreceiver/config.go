// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver"

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
	"go.opentelemetry.io/collector/config/confignet"
)

// Config defines the configurations for an AWS X-Ray receiver.
type Config struct {
	// The `NetAddr` represents the UDP address
	// and port on which this receiver listens for X-Ray segment documents
	// emitted by the X-Ray SDK.
	confignet.AddrConfig `mapstructure:",squash"`

	// ProxyServer defines configurations related to the local TCP proxy server.
	ProxyServer *proxy.Config `mapstructure:"proxy_server"`

	// Region specifies the AWS region to use for X-Ray service API calls
	Region string `mapstructure:"region"`
}

// LoadAWSConfig loads AWS SDK v2 configuration.
func (c *Config) LoadAWSConfig(ctx context.Context) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(c.Region))
	if err != nil {
		return aws.Config{}, err
	}
	return cfg, nil
}
