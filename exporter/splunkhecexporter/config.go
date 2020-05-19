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
)

// Config defines configuration for Splunk exporter.
type Config struct {
	configmodels.ExporterSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// HEC Token is the authentication token provided by Splunk.
	Token string `mapstructure:"token"`

	// URL is the Splunk HEC endpoint where data is going to be sent to.
	Url string `mapstructure:"url"`

	// Optional Splunk source: https://docs.splunk.com/Splexicon:Source.
	// Sources identify the incoming data.
	Source string `mapstructure:"source"`

	// Optional Splunk source type: https://docs.splunk.com/Splexicon:Sourcetype.
	SourceType string `mapstructure:"sourceType"`

	// Splunk index, optional name of the Splunk index.
	Index string `mapstructure:"index"`
}

func (cfg *Config) getOptionsFromConfig() (*exporterOptions, error) {
	if err := cfg.validateConfig(); err != nil {
		return nil, err
	}

	url, err := cfg.getUrl()
	if err != nil {
		return nil, fmt.Errorf("invalid \"url\": %v", err)
	}


	return &exporterOptions{
		Url:   url,
		Token: cfg.Token,
	}, nil
}

func (cfg *Config) validateConfig() error {
	if cfg.Url == "" {
		return errors.New("requires a non-empty \"url\"")
	}

	if cfg.Token == "" {
		return errors.New("requires a non-empty \"token\"")
	}

	return nil
}

func (cfg *Config) getUrl() (out *url.URL, err error) {

		out, err = url.Parse(cfg.Url)
		if err != nil {
			return out, err
		}
		if out.Path == "" || out.Path == "/" {
			out.Path = path.Join(out.Path, "services/collector")
		}

	return out, err
}
