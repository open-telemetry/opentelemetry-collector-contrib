// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/dockerobserver"

import (
	"errors"
	"fmt"
	"time"
)

// Config defines configuration for docker observer
type Config struct {

	// The URL of the docker server.  Default is "unix:///var/run/docker.sock"
	Endpoint string `mapstructure:"endpoint"`

	// The maximum amount of time to wait for docker API responses.  Default is 5s
	Timeout time.Duration `mapstructure:"timeout"`

	// A list of filters whose matching images are to be excluded.  Supports literals, globs, and regex.
	ExcludedImages []string `mapstructure:"excluded_images"`

	// If true, the "Config.Hostname" field (if present) of the docker
	// container will be used as the discovered host that is used to configure
	// receivers.  If false or if no hostname is configured, the field
	// `NetworkSettings.IPAddress` is used instead.
	UseHostnameIfPresent bool `mapstructure:"use_hostname_if_present"`

	// If true, the observer will configure receivers for matching container endpoints
	// using the host bound ip and port.  This is useful if containers exist that are not
	// accessible to an instance of the agent running outside of the docker network stack.
	// If UseHostnameIfPresent and this config are both enabled, this setting will take precedence.
	UseHostBindings bool `mapstructure:"use_host_bindings"`

	// If true, the observer will ignore discovered container endpoints that are not bound
	// to host ports.  This is useful if containers exist that are not accessible
	// to an instance of the agent running outside of the docker network stack.
	IgnoreNonHostBindings bool `mapstructure:"ignore_non_host_bindings"`

	// The time to wait before resyncing the list of containers the observer maintains
	// through the docker event listener example: cache_sync_interval: "20m"
	// Default: "60m"
	CacheSyncInterval time.Duration `mapstructure:"cache_sync_interval"`

	// Docker client API version. Default is 1.22
	DockerAPIVersion float64 `mapstructure:"api_version"`
}

func (config Config) Validate() error {
	if config.Endpoint == "" {
		return errors.New("endpoint must be specified")
	}
	if config.DockerAPIVersion < minimalRequiredDockerAPIVersion {
		return fmt.Errorf("api_version must be at least %v", minimalRequiredDockerAPIVersion)
	}
	if config.Timeout == 0 {
		return fmt.Errorf("timeout must be specified")
	}
	if config.CacheSyncInterval == 0 {
		return fmt.Errorf("cache_sync_interval must be specified")
	}
	return nil
}
