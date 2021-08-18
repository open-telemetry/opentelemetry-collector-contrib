// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docker

import (
	"errors"
	"time"
)

type Config struct {
	// The URL of the docker server. Default is "unix:///var/run/docker.sock"
	Endpoint string `mapstructure:"endpoint"`

	// The maximum amount of time to wait for docker API responses. Default is 5s
	Timeout time.Duration `mapstructure:"timeout"`

	// A list of filters whose matching images are to be excluded. Supports literals, globs, and regex.
	ExcludedImages []string `mapstructure:"excluded_images"`
}

// NewConfig creates a new config to be used when creating
// a docker client
func NewConfig(endpoint string, timeout time.Duration, excludedImages []string) (*Config, error) {
	cfg := &Config{
		Endpoint:       endpoint,
		Timeout:        timeout,
		ExcludedImages: excludedImages,
	}

	err := cfg.validate()
	return cfg, err
}

// NewDefaultConfig creates a new config with default values
// to be used when creating a docker client
func NewDefaultConfig() *Config {
	cfg := &Config{
		Endpoint: "unix:///var/run/docker.sock",
		Timeout:  5 * time.Second,
	}

	return cfg
}

// validate asserts that an endpoint field is set
// on the config struct
func (config Config) validate() error {
	if config.Endpoint == "" {
		return errors.New("config.Endpoint must be specified")
	}
	return nil
}
