// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"

import (
	"errors"
	"fmt"
	"time"
)

type Config struct {
	// The URL of the docker server. Default is "unix:///var/run/docker.sock"
	Endpoint string `mapstructure:"endpoint"`

	// The maximum amount of time to wait for docker API responses. Default is 5s
	Timeout time.Duration `mapstructure:"timeout"`

	// A list of filters whose matching images are to be excluded. Supports literals, globs, and regex.
	ExcludedImages []string `mapstructure:"excluded_images"`

	// Docker client API version.
	DockerAPIVersion float64 `mapstructure:"api_version"`
}

// NewConfig creates a new config to be used when creating
// a docker client
func NewConfig(endpoint string, timeout time.Duration, excludedImages []string, apiVersion float64) (*Config, error) {
	cfg := &Config{
		Endpoint:         endpoint,
		Timeout:          timeout,
		ExcludedImages:   excludedImages,
		DockerAPIVersion: apiVersion,
	}

	err := cfg.validate()
	return cfg, err
}

// NewDefaultConfig creates a new config with default values
// to be used when creating a docker client
func NewDefaultConfig() *Config {
	cfg := &Config{
		Endpoint:         "unix:///var/run/docker.sock",
		Timeout:          5 * time.Second,
		DockerAPIVersion: minimalRequiredDockerAPIVersion,
	}

	return cfg
}

// validate asserts that an endpoint field is set
// on the config struct
func (config Config) validate() error {
	if config.Endpoint == "" {
		return errors.New("config.Endpoint must be specified")
	}
	if config.DockerAPIVersion < minimalRequiredDockerAPIVersion {
		return fmt.Errorf("Docker API version must be at least %v", minimalRequiredDockerAPIVersion)
	}
	return nil
}
