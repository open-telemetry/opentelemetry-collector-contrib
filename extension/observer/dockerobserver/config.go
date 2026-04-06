// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/dockerobserver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
)

// Config defines configuration for docker observer
type Config struct {
	docker.Config `mapstructure:",squash"`

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
}

func (config Config) Validate() error {
	if config.DockerAPIVersion != "" {
		if err := docker.VersionIsValidAndGTE(config.DockerAPIVersion, minimumRequiredDockerAPIVersion); err != nil {
			return err
		}
	}
	if config.Timeout == 0 {
		return errors.New("timeout must be specified")
	}
	if config.CacheSyncInterval == 0 {
		return errors.New("cache_sync_interval must be specified")
	}
	return nil
}

func (config *Config) Unmarshal(conf *confmap.Conf) error {
	err := conf.Unmarshal(config)
	if err != nil {
		return err
	}

	if len(config.ExcludedImages) == 0 {
		config.ExcludedImages = nil
	}

	return err
}
