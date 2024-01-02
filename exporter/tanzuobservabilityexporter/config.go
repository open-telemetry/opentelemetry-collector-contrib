// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tanzuobservabilityexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter"

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type TracesConfig struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
}

type MetricsConfig struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	ResourceAttrsIncluded         bool `mapstructure:"resource_attrs_included"`
	// AppTagsExcluded will exclude the Resource Attributes `application`, `service.name` -> (service),
	// `cluster`, and `shard` from the transformed TObs metric if set to true.
	AppTagsExcluded bool `mapstructure:"app_tags_excluded"`
}

// Config defines configuration options for the exporter.
type Config struct {
	exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`

	// Traces defines the Traces exporter specific configuration
	Traces  TracesConfig  `mapstructure:"traces"`
	Metrics MetricsConfig `mapstructure:"metrics"`
}

func (c *Config) hasMetricsEndpoint() bool {
	return c.Metrics.Endpoint != ""
}

func (c *Config) hasTracesEndpoint() bool {
	return c.Traces.Endpoint != ""
}

func (c *Config) parseMetricsEndpoint() (hostName string, port int, err error) {
	return parseEndpoint(c.Metrics.Endpoint)
}

func (c *Config) parseTracesEndpoint() (hostName string, port int, err error) {
	return parseEndpoint(c.Traces.Endpoint)
}

func (c *Config) Validate() error {
	var tracesHostName, metricsHostName string
	var err error
	if c.hasTracesEndpoint() {
		tracesHostName, _, err = c.parseTracesEndpoint()
		if err != nil {
			return fmt.Errorf("failed to parse traces.endpoint: %w", err)
		}
	}
	if c.hasMetricsEndpoint() {
		metricsHostName, _, err = c.parseMetricsEndpoint()
		if err != nil {
			return fmt.Errorf("failed to parse metrics.endpoint: %w", err)
		}
	}
	if c.hasTracesEndpoint() && c.hasMetricsEndpoint() && tracesHostName != metricsHostName {
		return errors.New("host for metrics and traces must be the same")
	}
	return nil
}

func parseEndpoint(endpoint string) (hostName string, port int, err error) {
	if endpoint == "" {
		return "", 0, errors.New("a non-empty endpoint is required")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", 0, err
	}
	port, err = strconv.Atoi(u.Port())
	if err != nil {
		return "", 0, errors.New("valid port required")
	}
	hostName = u.Hostname()
	return hostName, port, nil
}
