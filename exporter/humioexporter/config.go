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

package humioexporter

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	basePath         = "api/v1/ingest/"
	unstructuredPath = basePath + "humio-unstructured"
	structuredPath   = basePath + "humio-structured"
)

// LogsConfig represents the Humio configuration settings specific to logs
type LogsConfig struct {
	//Ingest token for identifying and authorizing with a Humio repository
	IngestToken string `mapstructure:"ingest_token"`

	// The name of a custom log parser to use, if no parser is associated with the ingest token
	LogParser string `mapstructure:"log_parser"`
}

// TracesConfig represents the Humio configuration settings specific to traces
type TracesConfig struct {
	//Ingest token for identifying and authorizing with a Humio repository
	IngestToken string `mapstructure:"ingest_token"`

	// Whether to use Unix timestamps, or to fall back to ISO 8601 formatted strings
	UnixTimestamps bool `mapstructure:"unix_timestamps"`
}

// Config represents the Humio configuration settings
type Config struct {
	// Inherited settings
	config.ExporterSettings       `mapstructure:",squash"`
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`

	// Endpoint for the unstructured ingest API, created internally
	unstructuredEndpoint *url.URL

	// Endpoint for the structured ingest API, created internally
	structuredEndpoint *url.URL

	// Whether gzip compression should be disabled when sending data to Humio
	DisableCompression bool `mapstructure:"disable_compression"`

	// Name of tagging strategy used to target specific data sources for storage inside Humio
	Tag Tagger `mapstructure:"tag"`

	// Configuration options specific to logs
	Logs LogsConfig `mapstructure:"logs"`

	// Configuration options specific to traces
	Traces TracesConfig `mapstructure:"traces"`
}

// Validate ensures that a valid configuration has been provided, such that we can fail early
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("requires an endpoint")
	}

	if c.Tag != TagNone && c.Tag != TagTraceID && c.Tag != TagServiceName {
		return fmt.Errorf("tagging strategy must be one of %s, %s, or %s", TagNone, TagTraceID, TagServiceName)
	}

	// Ensure that it is possible to construct URLs to access the ingest API
	if _, err := c.getEndpoint(unstructuredPath); err != nil {
		return fmt.Errorf("unable to create URL for unstructured ingest API, endpoint %s is invalid", c.Endpoint)
	}

	headers := http.Header{}
	for k, v := range c.Headers {
		headers.Set(k, v)
	}

	// We require these headers, which should not be overwritten by the user
	if contentType := headers.Get("content-type"); contentType != "" && contentType != "application/json" {
		return errors.New("the Content-Type must be application/json, which is also the default for this header")
	}

	if authorization := headers.Get("authorization"); authorization != "" {
		return errors.New("the Authorization header must not be overwritten, since it is automatically generated from the ingest token")
	}

	if enc := headers.Get("content-encoding"); enc != "" && (c.DisableCompression || enc != "gzip") {
		return errors.New("the Content-Encoding header must be gzip when using compression, and empty when compression is disabled")
	}

	return nil
}

// Sanitize ensures that the correct headers are inserted and that a url for each endpoint is obtainable
func (c *Config) sanitize() error {
	structured, errS := c.getEndpoint(structuredPath)
	unstructured, errU := c.getEndpoint(unstructuredPath)

	if errS != nil || errU != nil {
		return fmt.Errorf("badly formatted endpoint %s", c.Endpoint)
	}
	c.structuredEndpoint = structured
	c.unstructuredEndpoint = unstructured

	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}

	c.Headers["content-type"] = "application/json"

	if !c.DisableCompression {
		c.Headers["content-encoding"] = "gzip"
	}

	if _, ok := c.Headers["user-agent"]; !ok {
		c.Headers["user-agent"] = "opentelemetry-collector-contrib Humio"
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
