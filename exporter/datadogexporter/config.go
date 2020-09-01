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
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/config/configmodels"
)

var (
	errUnsetAPIKey = errors.New("the Datadog API key is unset")
)

const (
	NoneMode      = "none"
	AgentlessMode = "agentless"
	DogStatsDMode = "dogstatsd"
)

// APIConfig defines the API configuration options
type APIConfig struct {
	// Key is the Datadog API key to associate your Agent's data with your organization.
	// Create a new API key here: https://app.datadoghq.com/account/settings
	Key string `mapstructure:"key"`

	// Site is the site of the Datadog intake to send data to.
	// The default value is "datadoghq.com".
	Site string `mapstructure:"site"`
}

// DogStatsDConfig defines the DogStatsd related configuration
type DogStatsDConfig struct {
	// Endpoint is the DogStatsD address.
	// The default value is 127.0.0.1:8125
	// A Unix address is supported
	Endpoint string `mapstructure:"endpoint"`

	// Telemetry states whether to send metrics
	Telemetry bool `mapstructure:"telemetry"`
}

// AgentlessConfig defines the Agentless related configuration
type AgentlessConfig struct {
	// Endpoint is the host of the Datadog intake server to send metrics to.
	// If unset, the value is obtained from the Site.
	Endpoint string `mapstructure:"endpoint"`
}

// MetricsConfig defines the metrics exporter specific configuration options
type MetricsConfig struct {
	// Namespace is the namespace under which the metrics are sent
	// By default metrics are not namespaced
	Namespace string `mapstructure:"namespace"`

	// Mode is the metrics sending mode: either 'dogstatsd' or 'agentless'
	Mode string `mapstructure:"mode"`

	// Percentiles states whether to report percentiles for summary metrics,
	// including the minimum and maximum
	Percentiles bool `mapstructure:"report_percentiles"`

	// Buckets states whether to report buckets from distribution metrics
	Buckets bool `mapstructure:"report_buckets"`

	// DogStatsD defines the DogStatsD configuration options.
	DogStatsD DogStatsDConfig `mapstructure:"dogstatsd"`

	// Agentless defines the Agentless configuration options.
	Agentless AgentlessConfig `mapstructure:"agentless"`
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

	// API defines the Datadog API configuration.
	API APIConfig `mapstructure:"api"`

	// Metrics defines the Metrics exporter specific configuration
	Metrics MetricsConfig `mapstructure:"metrics"`
}

// Sanitize tries to sanitize a given configuration
func (c *Config) Sanitize() error {

	if c.Metrics.Mode != AgentlessMode && c.Metrics.Mode != DogStatsDMode {
		return fmt.Errorf("Metrics mode '%s' is not recognized", c.Metrics.Mode)
	}

	// Exactly one configuration for metrics must be set
	if c.Metrics.Mode == AgentlessMode {
		if c.API.Key == "" {
			return errUnsetAPIKey
		}

		c.API.Key = strings.TrimSpace(c.API.Key)

		// Set the endpoint based on the Site
		if c.Metrics.Agentless.Endpoint == "" {
			c.Metrics.Agentless.Endpoint = fmt.Sprintf("https://api.%s", c.API.Site)
		}
	}

	return nil
}
