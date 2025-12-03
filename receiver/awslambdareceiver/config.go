// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

type Config struct {
	// Encoding defines the encoding to decode incoming Lambda invocation data.
	// This extension is expected to further process content of the events that are extracted from Lambda trigger.
	//
	// If receiving data is in different formats(ex:- a mix of VPC flow logs, CloudTrail logs), receiver is recommended
	// to have separate Lambda functions with specific extension configurations.
	Encoding string `mapstructure:"encoding"`

	_ struct{} // Prevent unkeyed literal initialization
}

var _ component.Config = (*Config)(nil)

func createDefaultConfig() component.Config {
	return &Config{}
}

func (c *Config) Validate() error {
	if c.Encoding == "" {
		return errors.New("encoding_extension is mandatory, please use a valid encoding extension name configured in the collector configurations")
	}

	return nil
}
