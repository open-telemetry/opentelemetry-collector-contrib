// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

var _ component.Config = (*Config)(nil)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	// The URL of the docker server.  Default is "unix:///var/run/docker.sock"
	Endpoint string `mapstructure:"endpoint"`

	// A mapping of container label names to MetricDescriptor label keys.
	// The corresponding container label value will become the DataPoint label value
	// for the mapped name.  E.g. `io.kubernetes.container.name: container_spec_name`
	// would result in a MetricDescriptor label called `container_spec_name` whose
	// Metric DataPoints have the value of the `io.kubernetes.container.name` container label.
	ContainerLabelsToMetricLabels map[string]string `mapstructure:"container_labels_to_metric_labels"`

	// A mapping of container environment variable names to MetricDescriptor label
	// keys.  The corresponding env var values become the DataPoint label value.
	// E.g. `APP_VERSION: version` would result MetricDescriptors having a label
	// key called `version` whose DataPoint label values are the value of the
	// `APP_VERSION` environment variable configured for that particular container, if
	// present.
	EnvVarsToMetricLabels map[string]string `mapstructure:"env_vars_to_metric_labels"`

	// A list of filters whose matching images are to be excluded.  Supports literals, globs, and regex.
	ExcludedImages []string `mapstructure:"excluded_images"`

	// Docker client API version. Default is 1.22
	DockerAPIVersion string `mapstructure:"api_version"`

	// MetricsBuilderConfig config. Enable or disable stats by name.
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
}

func (config Config) Validate() error {
	if config.Endpoint == "" {
		return errors.New("endpoint must be specified")
	}
	if err := docker.VersionIsValidAndGTE(config.DockerAPIVersion, minimumRequiredDockerAPIVersion); err != nil {
		return err
	}
	return nil
}

func (config *Config) Unmarshal(conf *confmap.Conf) error {
	err := conf.Unmarshal(config)
	if err != nil {
		if floatAPIVersion, ok := conf.Get("api_version").(float64); ok {
			return fmt.Errorf(
				"%w.\n\nHint: You may want to wrap the 'api_version' value in quotes (api_version: \"%1.2f\")",
				err,
				floatAPIVersion,
			)
		}
		return err
	}
	return nil
}
