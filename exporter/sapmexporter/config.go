// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sapmexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter"

import (
	"errors"
	"fmt"
	"net/url"

	sapmclient "github.com/signalfx/sapm-proto/client"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	defaultEndpointScheme = "https"
)

// Config defines configuration for SAPM exporter.
type Config struct {
	// Endpoint is the destination to where traces will be sent to in SAPM format.
	// It must be a full URL and include the scheme, port and path e.g, https://ingest.signalfx.com/v2/trace
	Endpoint string `mapstructure:"endpoint"`

	// AccessToken is the authentication token provided by SignalFx.
	AccessToken configopaque.String `mapstructure:"access_token"`

	// NumWorkers is the number of workers that should be used to export traces.
	// Exporter can make as many requests in parallel as the number of workers. Defaults to 8.
	NumWorkers uint `mapstructure:"num_workers"`

	// MaxConnections is used to set a limit to the maximum idle HTTP connection the exporter can keep open.
	MaxConnections uint `mapstructure:"max_connections"`

	// Disable compression. If set to true then Compression field is ignored.
	DisableCompression bool `mapstructure:"disable_compression"`

	// Compression method to use (gzip or zstd). Ignored if DisableCompression=true.
	// If unspecified defaults to gzip.
	Compression string `mapstructure:"compression"`

	// Log detailed response from trace ingest.
	LogDetailedResponse bool `mapstructure:"log_detailed_response"`

	splunk.AccessTokenPassthroughConfig `mapstructure:",squash"`

	TimeoutSettings           exporterhelper.TimeoutConfig    `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("`endpoint` not specified")
	}
	_, err := url.Parse(c.Endpoint)
	if err != nil {
		return err
	}

	switch c.Compression {
	// Valid compression methods.
	case "", // no compression
		string(sapmclient.CompressionMethodGzip),
		string(sapmclient.CompressionMethodZstd):

	default:
		return fmt.Errorf("invalid compression %q", c.Compression)
	}

	return nil
}

func (c *Config) clientOptions() []sapmclient.Option {
	e, _ := url.Parse(c.Endpoint)
	endpoint := c.Endpoint
	if e.Scheme == "" {
		e.Scheme = defaultEndpointScheme
		endpoint = e.String()
	}
	opts := []sapmclient.Option{
		sapmclient.WithEndpoint(endpoint),
	}
	if c.NumWorkers > 0 {
		opts = append(opts, sapmclient.WithWorkers(c.NumWorkers))
	}

	if c.MaxConnections > 0 {
		opts = append(opts, sapmclient.WithMaxConnections(c.MaxConnections))
	}

	if c.AccessToken != "" {
		opts = append(opts, sapmclient.WithAccessToken(string(c.AccessToken)))
	}

	if c.DisableCompression {
		opts = append(opts, sapmclient.WithDisabledCompression())
	}

	if c.Compression != "" {
		opts = append(opts, sapmclient.WithCompressionMethod(sapmclient.CompressionMethod(c.Compression)))
	}

	return opts
}
