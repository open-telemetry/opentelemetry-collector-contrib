// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"fmt"
	"regexp"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

var _ component.Config = (*Config)(nil)

type Config struct {
	docker.Config `mapstructure:",squash"`

	scraperhelper.ControllerConfig `mapstructure:",squash"`

	// A mapping of container label names to MetricDescriptor label keys.
	// The corresponding container label value will become the DataPoint label value
	// for the mapped name.  E.g. `io.kubernetes.container.name: container_spec_name`
	// would result in a MetricDescriptor label called `container_spec_name` whose
	// Metric DataPoints have the value of the `io.kubernetes.container.name` container label.
	ContainerLabelsToMetricLabels map[string]string `mapstructure:"container_labels_to_metric_labels"`

	// An OR'ed allow list of matchers to pass container label names though as ResourceAttributes.
	ContainerLabelsToResourceAttributes []LabelMatcher `mapstructure:"container_labels_to_resource_attributes"`

	// A mapping of container environment variable names to MetricDescriptor label
	// keys.  The corresponding env var values become the DataPoint label value.
	// E.g. `APP_VERSION: version` would result MetricDescriptors having a label
	// key called `version` whose DataPoint label values are the value of the
	// `APP_VERSION` environment variable configured for that particular container, if
	// present.
	EnvVarsToMetricLabels map[string]string `mapstructure:"env_vars_to_metric_labels"`

	// MetricsBuilderConfig config. Enable or disable stats by name.
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
}

// matchType is the enum to capture the two types of allows matches.
type matchType string

const (
	// strictMatchType is the MatchType for filtering by exact string matches.
	strictMatchType matchType = "strict"

	// regexpMatchType is the MatchType for filtering by regexp string matches.
	regexpMatchType matchType = "regexp"
)

var matchTypes = []matchType{strictMatchType, regexpMatchType}

func (mt matchType) isValid() bool {
	for _, matchType := range matchTypes {
		if mt == matchType {
			return true
		}
	}

	return false
}

// LabelMatcher represents a matcher for container label values.
type LabelMatcher struct {
	MatchType matchType `mapstructure:"match_type"`
	Include   string    `mapstructure:"include"`
}

func (config Config) Validate() error {
	if err := docker.VersionIsValidAndGTE(config.DockerAPIVersion, minimumRequiredDockerAPIVersion); err != nil {
		return err
	}

	if config.ContainerLabelsToResourceAttributes != nil {
		for _, lm := range config.ContainerLabelsToResourceAttributes {
			_, compileErr := regexp.Compile(lm.Include)
			switch {
			case lm.MatchType == "":
				return fmt.Errorf("match_type is required for container_labels_to_resource_attributes entries")
			case !lm.MatchType.isValid():
				return fmt.Errorf("match_type must be one of 'strict' or 'regex' for container_labels_to_resource_attributes entries")
			case lm.Include == "":
				return fmt.Errorf("include is required for container_labels_to_resource_attributes entries")
			case lm.MatchType == regexpMatchType && compileErr != nil:
				return fmt.Errorf("include regex for container_labels_to_resource_attributes entries can not be compiled: %w", compileErr)
			}
		}
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
