// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanagerprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerprovider"

import (
	"errors"
	"time"
)

const defaultRefreshInterval = 60 * time.Minute

var (
	errMissingSecretARN        = errors.New("secret_arn is required")
	errMissingRegion           = errors.New("region is required")
	errNegativeRefreshInterval = errors.New("refresh_interval must not be negative")
)

type Config struct {
	SecretARN       string        `mapstructure:"secret_arn"`
	Region          string        `mapstructure:"region"`
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
}

func (cfg *Config) Validate() error {
	if cfg.SecretARN == "" {
		return errMissingSecretARN
	}
	if cfg.Region == "" {
		return errMissingRegion
	}
	if cfg.RefreshInterval < 0 {
		return errNegativeRefreshInterval
	}
	return nil
}
