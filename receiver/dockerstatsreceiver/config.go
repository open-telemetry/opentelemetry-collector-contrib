// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

var _ component.Config = (*Config)(nil)

// EventsConfig contains configuration for Docker events collection
type EventsConfig struct {
	// Filters allows filtering which Docker events to collect
	Filters map[string][]string `mapstructure:"filters"`
	// Since shows events created since this timestamp
	Since string `mapstructure:"since"`
	// Until shows events created until this timestamp
	Until string `mapstructure:"until"`
}

type Config struct {
	docker.Config `mapstructure:",squash"`

	// Metrics-specific settings

	scraperhelper.ControllerConfig `mapstructure:",squash"`

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

	// MetricsBuilderConfig config. Enable or disable stats by name.
	metadata.MetricsBuilderConfig `mapstructure:",squash"`

	// Logs-specific settings
	// MinDockerRetryWait is the minimum time to wait before retrying to connect to the Docker daemon
	MinDockerRetryWait time.Duration `mapstructure:"min_docker_retry_wait"`
	// MaxDockerRetryWait is the maximum time to wait before retrying to connect to the Docker daemon
	MaxDockerRetryWait time.Duration `mapstructure:"max_docker_retry_wait"`

	// Logs configuration (Docker events)
	Logs EventsConfig `mapstructure:"logs"`
}

func parseTimestamp(ts string) (time.Time, error) {
	// Try Unix timestamp first
	if i, err := strconv.ParseInt(ts, 10, 64); err == nil {
		return time.Unix(i, 0), nil
	}

	// Try RFC3339
	return time.Parse(time.RFC3339, ts)
}

func (config Config) Validate() error {
	if err := docker.VersionIsValidAndGTE(config.DockerAPIVersion, minimumRequiredDockerAPIVersion); err != nil {
		return err
	}

	// Validate logs-specific config
	if config.MinDockerRetryWait <= 0 {
		return fmt.Errorf("min_docker_retry_wait must be positive, got %v", config.MinDockerRetryWait)
	}
	if config.MaxDockerRetryWait <= 0 {
		return fmt.Errorf("max_docker_retry_wait must be positive, got %v", config.MaxDockerRetryWait)
	}
	if config.MaxDockerRetryWait < config.MinDockerRetryWait {
		return fmt.Errorf("max_docker_retry_wait must not be less than min_docker_retry_wait")
	}

	now := time.Now()
	var sinceTime time.Time
	if config.Logs.Since != "" {
		var err error
		sinceTime, err = parseTimestamp(config.Logs.Since)
		if err != nil {
			return fmt.Errorf("logs.since must be a Unix timestamp or RFC3339 time: %w", err)
		}
		if sinceTime.After(now) {
			return fmt.Errorf("logs.since cannot be in the future")
		}
	}

	// Parse and validate until if set
	var untilTime time.Time
	if config.Logs.Until != "" {
		var err error
		untilTime, err = parseTimestamp(config.Logs.Until)
		if err != nil {
			return fmt.Errorf("logs.until must be a Unix timestamp or RFC3339 time: %w", err)
		}
		if untilTime.After(now) {
			config.Logs.Until = "" // Clear future until time
		}
	}

	// If both are set, ensure since is not after until
	if config.Logs.Since != "" && config.Logs.Until != "" {
		if sinceTime.After(untilTime) {
			return fmt.Errorf("logs.since must not be after logs.until")
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
