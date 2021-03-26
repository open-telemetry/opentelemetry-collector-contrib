// Copyright 2021, OpenTelemetry Authors
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

package humioexporter

import (
	"errors"
	"net/url"
	"path"

	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	basePath         = "api/v1/ingest/"
	unstructuredPath = basePath + "humio-unstructured"
	structuredPath   = basePath + "humio-structured"
)

// Humio configuration settings specific to logs
type LogsConfig struct {
	// The name of a custom log parser to use, if no parser is associated with the ingest token
	LogParser string `mapstructure:"log_parser"`
}

// Humio configuration settings specific to traces
type TracesConfig struct {
	// Whether to use ISO 8601 formatted timestamps, or to fall back to Unix time
	IsoTimestamps bool `mapstructure:"iso_timestamps"`

	// The time zone to use when representing timestamps in Unix time
	TimeZone string `mapstructure:"timezone"`

	// Whether to attach a raw string representation of traces when exporting events to Humio
	EnableRawstrings bool `mapstructure:"enable_rawstrings"`
}

// Humio configuration settings
type Config struct {
	// Squash ensures fields are correctly decoded in embedded struct
	configmodels.ExporterSettings `mapstructure:",squash"`

	// Settings inherited from exporter helper
	exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`

	//Ingest token for identifying and authorizing with a Humio repository
	IngestToken string `mapstructure:"ingest_token"`

	// The base URL on which the Humio backend can be reached
	Endpoint string `mapstructure:"endpoint"`

	// Key-value pairs used to target specific data sources for storage inside Humio
	Tags map[string]string `mapstructure:"tags,omitempty"`

	// Whether this exporter should automatically add the service name as a tag
	EnableServiceTag bool `mapstructure:"enable_service_tag"`

	// Configuration options specific to logs
	Logs LogsConfig `mapstructure:"logs"`

	// Configuration options specific to traces
	Traces TracesConfig `mapstructure:"traces"`
}

// Ensure that a valid configuration has been provided, such that we can fail early
func (c *Config) validate() error {
	if c.IngestToken == "" {
		return errors.New("requires an ingest_token")
	}

	if c.Endpoint == "" {
		return errors.New("requires an endpoint")
	}

	if !c.EnableServiceTag && len(c.Tags) == 0 {
		return errors.New("requires at least one custom tag when disabling service tag")
	}

	if !c.Traces.IsoTimestamps && c.Traces.TimeZone == "" {
		return errors.New("requires a time zone when using Unix timestamps")
	}

	return nil
}

// Get a URL for a specific destination path on the Humio endpoint
func (c *Config) getEndpoint(dest string) (*url.URL, error) {
	res, err := url.Parse(c.Endpoint)
	if err != nil {
		return res, err
	}

	res.Path = path.Join(res.Path, dest)
	return res, nil
}
