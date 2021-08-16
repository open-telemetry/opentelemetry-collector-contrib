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

package sapmexporter

import (
	"errors"
	"net/url"

	sapmclient "github.com/signalfx/sapm-proto/client"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	defaultEndpointScheme = "https"
	defaultNumWorkers     = 8
)

// Config defines configuration for SAPM exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`

	// Endpoint is the destination to where traces will be sent to in SAPM format.
	// It must be a full URL and include the scheme, port and path e.g, https://ingest.signalfx.com/v2/trace
	Endpoint string `mapstructure:"endpoint"`

	// AccessToken is the authentication token provided by SignalFx.
	AccessToken string `mapstructure:"access_token"`

	// NumWorkers is the number of workers that should be used to export traces.
	// Exporter can make as many requests in parallel as the number of workers. Defaults to 8.
	NumWorkers uint `mapstructure:"num_workers"`

	// MaxConnections is used to set a limit to the maximum idle HTTP connection the exporter can keep open.
	MaxConnections uint `mapstructure:"max_connections"`

	// Disable GZip compression.
	DisableCompression bool `mapstructure:"disable_compression"`

	splunk.AccessTokenPassthroughConfig `mapstructure:",squash"`

	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
}

func (c *Config) validate() error {
	if c.Endpoint == "" {
		return errors.New("`endpoint` not specified")
	}

	e, err := url.Parse(c.Endpoint)
	if err != nil {
		return err
	}

	if e.Scheme == "" {
		e.Scheme = defaultEndpointScheme
	}
	c.Endpoint = e.String()
	return nil
}

func (c *Config) clientOptions() []sapmclient.Option {
	opts := []sapmclient.Option{
		sapmclient.WithEndpoint(c.Endpoint),
	}
	if c.NumWorkers > 0 {
		opts = append(opts, sapmclient.WithWorkers(c.NumWorkers))
	}

	if c.MaxConnections > 0 {
		opts = append(opts, sapmclient.WithMaxConnections(c.MaxConnections))
	}

	if c.AccessToken != "" {
		opts = append(opts, sapmclient.WithAccessToken(c.AccessToken))
	}

	if c.DisableCompression {
		opts = append(opts, sapmclient.WithDisabledCompression())
	}

	return opts
}
