// Copyright The OpenTelemetry Authors
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

package datadogexporter

import (
	"fmt"

	"go.opentelemetry.io/collector/config/configmodels"
)

// DogStatsDConfig defines the DogStatsd related configuration
type DogStatsDConfig struct {
	// Endpoint is the DogStatsD address.
	// The default value is 127.0.0.1:8125
	// A Unix address is supported
	Endpoint string `mapstructure:"endpoint"`

	// Telemetry states whether to send internal telemetry metrics from the statsd client
	Telemetry bool `mapstructure:"telemetry"`
}

// MetricsConfig defines the metrics exporter specific configuration options
type MetricsConfig struct {
	// Namespace is the namespace under which the metrics are sent
	// By default metrics are not namespaced
	Namespace string `mapstructure:"namespace"`

	// Percentiles states whether to report percentiles for summary metrics,
	// including the minimum and maximum
	Percentiles bool `mapstructure:"report_percentiles"`

	// Buckets states whether to report buckets from distribution metrics
	Buckets bool `mapstructure:"report_buckets"`

	// DogStatsD defines the DogStatsD configuration options.
	DogStatsD DogStatsDConfig `mapstructure:"dogstatsd"`
}

// TagsConfig defines the tag-related configuration
// It is embedded in the configuration
type TagsConfig struct {
	// Hostname is the host name for unified service tagging.
	// If unset, it is determined automatically.
	// See https://docs.datadoghq.com/agent/faq/how-datadog-agent-determines-the-hostname
	// for more details.
	Hostname string `mapstructure:"hostname"`

	// Env is the environment for unified service tagging.
	// It can also be set through the `DD_ENV` environment variable.
	Env string `mapstructure:"env"`

	// Service is the service for unified service tagging.
	// It can also be set through the `DD_SERVICE` environment variable.
	Service string `mapstructure:"service"`

	// Version is the version for unified service tagging.
	// It can also be set through the `DD_VERSION` version variable.
	Version string `mapstructure:"version"`

	// Tags is the list of default tags to add to every metric or trace.
	Tags []string `mapstructure:"tags"`
}

// GetTags gets the default tags extracted from the configuration
func (t *TagsConfig) GetTags() []string {
	tags := make([]string, 0, 4)

	if t.Hostname != "" {
		tags = append(tags, fmt.Sprintf("host:%s", t.Hostname))
	}

	if t.Env != "" {
		tags = append(tags, fmt.Sprintf("env:%s", t.Env))
	}

	if t.Service != "" {
		tags = append(tags, fmt.Sprintf("service:%s", t.Service))
	}

	if t.Version != "" {
		tags = append(tags, fmt.Sprintf("version:%s", t.Version))
	}

	if len(t.Tags) > 0 {
		tags = append(tags, t.Tags...)
	}

	return tags
}

// Config defines configuration for the Datadog exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	TagsConfig `mapstructure:",squash"`

	// Metrics defines the Metrics exporter specific configuration
	Metrics MetricsConfig `mapstructure:"metrics"`
}

// Sanitize tries to sanitize a given configuration
func (c *Config) Sanitize() error {
	// This will be useful on a future PR
	return nil
}
