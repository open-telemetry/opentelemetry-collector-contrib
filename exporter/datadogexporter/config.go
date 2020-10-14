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
	"go.opentelemetry.io/collector/config/confignet"
)

var (
	errUnsetAPIKey = errors.New("api.key is not set")
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

// GetCensoredKey returns the API key censored for logging purposes
func (api *APIConfig) GetCensoredKey() string {
	if len(api.Key) <= 5 {
		return api.Key
	}
	return strings.Repeat("*", len(api.Key)-5) + api.Key[len(api.Key)-5:]
}

// MetricsConfig defines the metrics exporter specific configuration options
type MetricsConfig struct {
	// Namespace is the namespace under which the metrics are sent
	// By default metrics are not namespaced
	Namespace string `mapstructure:"namespace"`

	// Buckets states whether to report buckets from distribution metrics
	Buckets bool `mapstructure:"report_buckets"`

	// TCPAddr.Endpoint is the host of the Datadog intake server to send metrics to.
	// If unset, the value is obtained from the Site.
	confignet.TCPAddr `mapstructure:",squash"`
}

// TracesConfig defines the traces exporter specific configuration options
type TracesConfig struct {
	// TCPAddr.Endpoint is the host of the Datadog intake server to send traces to.
	// If unset, the value is obtained from the Site.
	confignet.TCPAddr `mapstructure:",squash"`

	// SampleRate is the rate at which to sample this event. Default is 1,
	// meaning no sampling. If you want to send one event out of every 250
	// times Send() is called, you would specify 250 here.
	SampleRate uint `mapstructure:"sample_rate"`
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
func (t *TagsConfig) GetTags(addHost bool) []string {
	tags := make([]string, 0, 4)

	vars := map[string]string{
		"env":     t.Env,
		"service": t.Service,
		"version": t.Version,
	}

	if addHost {
		vars["host"] = t.Hostname
	}

	for name, val := range vars {
		if val != "" {
			tags = append(tags, fmt.Sprintf("%s:%s", name, val))
		}
	}

	tags = append(tags, t.Tags...)

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

	// Traces defines the Traces exporter specific configuration
	Traces TracesConfig `mapstructure:"traces"`
}

// Sanitize tries to sanitize a given configuration
func (c *Config) Sanitize() error {
	// Add '.' at the end of namespace
	if c.Metrics.Namespace != "" && !strings.HasSuffix(c.Metrics.Namespace, ".") {
		c.Metrics.Namespace = c.Metrics.Namespace + "."
	}

	if c.TagsConfig.Env == "" {
		c.TagsConfig.Env = "none"
	}

	if c.API.Key == "" {
		return errUnsetAPIKey
	}

	c.API.Key = strings.TrimSpace(c.API.Key)

	// Set the endpoint based on the Site
	if c.Metrics.TCPAddr.Endpoint == "" {
		c.Metrics.TCPAddr.Endpoint = fmt.Sprintf("https://api.%s", c.API.Site)
	}

	if c.Traces.TCPAddr.Endpoint == "" {
		c.Traces.TCPAddr.Endpoint = fmt.Sprintf("https://trace.agent.%s", c.API.Site)
	}

	return nil
}
