// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"errors"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
)

const (
	defaultOpenSearchEnvName = "OPENSEARCH_URL"
)

// Config defines configuration for OpenSearch exporter.
type Config struct {

	// Endpoints holds the OpenSearch URLs the exporter should send events to.
	//
	// OPENSEARCH_URL environment variable is not set.
	Endpoints []string `mapstructure:"endpoints"`

	// NumWorkers configures the number of workers publishing bulk requests.
	NumWorkers int `mapstructure:"num_workers"`

	// This setting is required when logging pipelines used.
	LogsIndex string `mapstructure:"logs_index"`

	// This setting is required when traces pipelines used.
	TracesIndex string `mapstructure:"traces_index"`

	// Pipeline ingest node pipeline name that should be used to process the
	// events.
	Pipeline string `mapstructure:"pipeline"`

	HTTPClientSettings `mapstructure:",squash"`
	Retry              RetrySettings    `mapstructure:"retry"`
	Flush              FlushSettings    `mapstructure:"flush"`
	Mapping            MappingsSettings `mapstructure:"mapping"`
}

type HTTPClientSettings struct {
	Authentication AuthenticationSettings `mapstructure:",squash"`

	// ReadBufferSize for HTTP client. See http.Transport.ReadBufferSize.
	ReadBufferSize int `mapstructure:"read_buffer_size"`

	// WriteBufferSize for HTTP client. See http.Transport.WriteBufferSize.
	WriteBufferSize int `mapstructure:"write_buffer_size"`

	// Timeout configures the HTTP request timeout.
	Timeout time.Duration `mapstructure:"timeout"`

	// Headers allows users to configure optional HTTP headers that
	// will be send with each HTTP request.
	Headers map[string]string `mapstructure:"headers,omitempty"`

	configtls.TLSClientSetting `mapstructure:"tls,omitempty"`
}

// AuthenticationSettings defines user authentication related settings.
type AuthenticationSettings struct {
	// User is used to configure HTTP Basic Authentication.
	User string `mapstructure:"user"`

	// Password is used to configure HTTP Basic Authentication.
	Password configopaque.String `mapstructure:"password"`
}

// FlushSettings  defines settings for configuring the write buffer flushing
// policy in the OpenSearch exporter. The exporter sends a bulk request with
// all events already serialized into the send-buffer.
type FlushSettings struct {
	// Bytes sets the send buffer flushing limit.
	Bytes int `mapstructure:"bytes"`

	// Interval configures the max age of a document in the send buffer.
	Interval time.Duration `mapstructure:"interval"`
}

// RetrySettings defines settings for the HTTP request retries in the OpenSearch exporter.
// Failed sends are retried with exponential backoff.
type RetrySettings struct {
	// Enabled allows users to disable retry without having to comment out all settings.
	Enabled bool `mapstructure:"enabled"`

	// MaxRequests configures how often an HTTP request is retried before it is assumed to be failed.
	MaxRequests int `mapstructure:"max_requests"`

	// InitialInterval configures the initial waiting time if a request failed.
	InitialInterval time.Duration `mapstructure:"initial_interval"`

	// MaxInterval configures the max waiting time if consecutive requests failed.
	MaxInterval time.Duration `mapstructure:"max_interval"`
}

type MappingsSettings struct {
	// Mode configures the field mappings.
	Mode string `mapstructure:"mode"`

	// Try to find and remove duplicate fields
	Dedup bool `mapstructure:"dedup"`

	Dedot bool `mapstructure:"dedot"`
}

var (
	errConfigNoEndpoint    = errors.New("endpoints must be specified")
	errConfigEmptyEndpoint = errors.New("endpoints must not include empty entries")
)

func isValidMapping(candidate string) bool {
	switch candidate {
	case "none":
	case "no":
	case "sso":
		return true
	}
	return false
}

// Validate validates the opensearch server configuration.
func (cfg *Config) Validate() error {
	if len(cfg.Endpoints) == 0 {
		if os.Getenv(defaultOpenSearchEnvName) == "" {
			return errConfigNoEndpoint
		}
	}

	for _, endpoint := range cfg.Endpoints {
		if endpoint == "" {
			return errConfigEmptyEndpoint
		}
	}

	if !isValidMapping(cfg.Mapping.Mode) {
		return fmt.Errorf("unknown mapping mode %v", cfg.Mapping.Mode)
	}

	return nil
}
