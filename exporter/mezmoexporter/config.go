// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mezmoexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mezmoexporter"

import (
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// defaultTimeout
	defaultTimeout time.Duration = 5 * time.Second

	// defaultIngestURL
	defaultIngestURL = "https://logs.mezmo.com/otel/ingest/rest"

	// See https://docs.mezmo.com/docs/Mezmo-ingestion-service-limits for details

	// Maximum payload in bytes that can be POST'd to the REST endpoint
	maxBodySize     = 10 * 1024 * 1024
	maxMessageSize  = 16 * 1024
	maxMetaDataSize = 32 * 1024
	maxAppnameLen   = 512
	maxLogLevelLen  = 80
)

// Config defines configuration for Mezmo exporter.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`

	// IngestURL is the URL to send telemetry to.
	IngestURL string `mapstructure:"ingest_url"`

	// Token is the authentication token provided by Mezmo.
	IngestKey configopaque.String `mapstructure:"ingest_key"`
}

// returns default http client settings
func createDefaultHTTPClientSettings() confighttp.HTTPClientSettings {
	return confighttp.HTTPClientSettings{
		Timeout: defaultTimeout,
	}
}

func (c *Config) Validate() error {
	var err error
	var parsed *url.URL

	parsed, err = url.Parse(c.IngestURL)
	if c.IngestURL == "" || err != nil {
		return fmt.Errorf(`"ingest_url" must be a valid URL`)
	}

	if parsed.Host == "" {
		return fmt.Errorf(`"ingest_url" must contain a valid host`)
	}

	return nil
}
