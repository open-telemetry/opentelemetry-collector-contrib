// Copyright 2019, OpenTelemetry Authors
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

package signalfxexporter

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"time"

	"go.opentelemetry.io/collector/config/configmodels"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

// Config defines configuration for SignalFx exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// AccessToken is the authentication token provided by SignalFx.
	AccessToken string `mapstructure:"access_token"`

	// Realm is the SignalFx realm where data is going to be sent to. The
	// default value is "us0"
	Realm string `mapstructure:"realm"`

	// IngestURL is the destination to where SignalFx metrics will be sent to, it is
	// intended for tests and debugging. The value of Realm is ignored if the
	// URL is specified. If a path is not included the exporter will
	// automatically append the appropriate path, eg.: "v2/datapoint".
	// If a path is specified it will use the one set by the config.
	IngestURL string `mapstructure:"ingest_url"`

	// APIURL is the destination to where SignalFx metadata will be sent. This
	// value takes precedence over the value of Realm
	APIURL string `mapstructure:"api_url"`

	// Timeout is the maximum timeout for HTTP request sending trace data. The
	// default value is 5 seconds.
	Timeout time.Duration `mapstructure:"timeout"`

	// Headers are a set of headers to be added to the HTTP request sending
	// trace data. These can override pre-defined header values used by the
	// exporter, eg: "User-Agent" can be set to a custom value if specified
	// here.
	Headers map[string]string `mapstructure:"headers"`

	// Whether to log dimension updates being sent to SignalFx.
	LogDimensionUpdates bool `mapstructure:"log_dimension_updates"`

	splunk.AccessTokenPassthroughConfig `mapstructure:",squash"`
}

func (cfg *Config) getOptionsFromConfig() (*exporterOptions, error) {
	if err := cfg.validateConfig(); err != nil {
		return nil, err
	}

	ingestURL, err := cfg.getIngestURL()
	if err != nil {
		return nil, fmt.Errorf("invalid \"ingest_url\": %v", err)
	}

	apiURL, err := cfg.getAPIURL()
	if err != nil {
		return nil, fmt.Errorf("invalid \"api_url\": %v", err)
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}

	return &exporterOptions{
		ingestURL:    ingestURL,
		apiURL:       apiURL,
		httpTimeout:  cfg.Timeout,
		token:        cfg.AccessToken,
		logDimUpdate: cfg.LogDimensionUpdates,
	}, nil
}

func (cfg *Config) validateConfig() error {
	if cfg.AccessToken == "" {
		return errors.New("requires a non-empty \"access_token\"")
	}

	if cfg.Realm == "" && (cfg.IngestURL == "" || cfg.APIURL == "") {
		return errors.New("requires a non-empty \"realm\", or" +
			" \"ingest_url\" and \"api_url\" should be explicitly set")
	}

	if cfg.Timeout < 0 {
		return errors.New("cannot have a negative \"timeout\"")
	}

	return nil
}

func (cfg *Config) getIngestURL() (out *url.URL, err error) {
	if cfg.IngestURL == "" {
		out, err = url.Parse(fmt.Sprintf("https://ingest.%s.signalfx.com/v2/datapoint", cfg.Realm))
		if err != nil {
			return out, err
		}
	} else {
		// Ignore realm and use the IngestURL. Typically used for debugging.
		out, err = url.Parse(cfg.IngestURL)
		if err != nil {
			return out, err
		}
		if out.Path == "" || out.Path == "/" {
			out.Path = path.Join(out.Path, "v2/datapoint")
		}
	}

	return out, err
}

func (cfg *Config) getAPIURL() (*url.URL, error) {
	if cfg.APIURL == "" {
		return url.Parse(fmt.Sprintf("https://api.%s.signalfx.com", cfg.Realm))
	}
	return url.Parse(cfg.APIURL)
}
