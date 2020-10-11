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

package splunkhecexporter

import (
	"errors"
	"fmt"
	"net/url"
	"path"

	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// hecPath is the default HEC path on the Splunk instance.
	hecPath = "services/collector"
)

// Config defines configuration for Splunk exporter.
type Config struct {
	configmodels.ExporterSettings  `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	// HEC Token is the authentication token provided by Splunk.
	Token string `mapstructure:"token"`

	// URL is the Splunk HEC endpoint where data is going to be sent to.
	Endpoint string `mapstructure:"endpoint"`

	// Optional Splunk source: https://docs.splunk.com/Splexicon:Source.
	// Sources identify the incoming data.
	Source string `mapstructure:"source"`

	// Optional Splunk source type: https://docs.splunk.com/Splexicon:Sourcetype.
	SourceType string `mapstructure:"sourcetype"`

	// Splunk index, optional name of the Splunk index.
	Index string `mapstructure:"index"`

	// MaxConnections is used to set a limit to the maximum idle HTTP connection the exporter can keep open. Defaults to 100.
	MaxConnections uint `mapstructure:"max_connections"`

	// Disable GZip compression. Defaults to false.
	DisableCompression bool `mapstructure:"disable_compression"`

	// insecure_skip_verify skips checking the certificate of the HEC endpoint when sending data over HTTPS. Defaults to false.
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}

func (cfg *Config) getOptionsFromConfig() (*exporterOptions, error) {
	if err := cfg.validateConfig(); err != nil {
		return nil, err
	}

	url, err := cfg.getURL()
	if err != nil {
		return nil, fmt.Errorf(`invalid "endpoint": %v`, err)
	}

	return &exporterOptions{
		url:   url,
		token: cfg.Token,
	}, nil
}

func (cfg *Config) validateConfig() error {
	if cfg.Endpoint == "" {
		return errors.New(`requires a non-empty "endpoint"`)
	}

	if cfg.Token == "" {
		return errors.New(`requires a non-empty "token"`)
	}

	return nil
}

func (cfg *Config) getURL() (out *url.URL, err error) {

	out, err = url.Parse(cfg.Endpoint)
	if err != nil {
		return out, err
	}
	if out.Path == "" || out.Path == "/" {
		out.Path = path.Join(out.Path, hecPath)
	}

	return
}
