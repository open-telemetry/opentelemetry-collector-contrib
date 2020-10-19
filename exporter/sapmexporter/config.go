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
	"time"

	sapmclient "github.com/signalfx/sapm-proto/client"
	"github.com/signalfx/signalfx-agent/pkg/apm/correlations"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

const (
	defaultEndpointScheme = "https"
	defaultNumWorkers     = 8
)

// CorrelationConfig defines correlation settings.
type CorrelationConfig struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	correlations.Config           `mapstructure:",squash"`
	// Enabled determines whether correlation is enabled or not.
	Enabled bool `mapstructure:"enabled"`
	// How long to wait after a trace span's service name is last seen before
	// uncorrelating that service.
	StaleServiceTimeout time.Duration `mapstructure:"stale_service_timeout"`
	// SyncAttributes is a key of the span attribute name to sync to the dimension as the value.
	SyncAttributes map[string]string `mapstructure:"sync_attributes"`
}

// Config defines configuration for SAPM exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// Endpoint is the destination to where traces will be sent to in SAPM format.
	// It must be a full URL and include the scheme, port and path e.g, https://ingest.signalfx.com/v2/trace
	Endpoint string `mapstructure:"endpoint"`

	// Correlation settings for associating environment and services observed from traces to metrics.
	Correlation CorrelationConfig `mapstructure:"correlation"`

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

	if c.Correlation.Enabled && c.Correlation.Endpoint == "" {
		return errors.New("`correlation.endpoint` must be set when `correlation.enabled` is true")
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
