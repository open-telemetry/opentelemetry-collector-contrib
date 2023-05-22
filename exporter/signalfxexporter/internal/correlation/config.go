// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package correlation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/correlation"

import (
	"errors"
	"net/url"
	"time"

	"github.com/signalfx/signalfx-agent/pkg/apm/correlations"
	"go.opentelemetry.io/collector/config/confighttp"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

// DefaultConfig returns default configuration correlation values.
func DefaultConfig() *Config {
	return &Config{
		HTTPClientSettings:  confighttp.HTTPClientSettings{Timeout: 5 * time.Second},
		StaleServiceTimeout: 5 * time.Minute,
		SyncAttributes: map[string]string{
			conventions.AttributeK8SPodUID:   conventions.AttributeK8SPodUID,
			conventions.AttributeContainerID: conventions.AttributeContainerID,
		},
		Config: correlations.Config{
			MaxRequests:     20,
			MaxBuffered:     10_000,
			MaxRetries:      2,
			LogUpdates:      false,
			RetryDelay:      30 * time.Second,
			CleanupInterval: 1 * time.Minute,
		},
	}
}

// Config defines configuration for correlation via traces.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	correlations.Config           `mapstructure:",squash"`

	// How long to wait after a trace span's service name is last seen before
	// uncorrelating that service.
	StaleServiceTimeout time.Duration `mapstructure:"stale_service_timeout"`
	// SyncAttributes is a key of the span attribute name to sync to the dimension as the value.
	SyncAttributes map[string]string `mapstructure:"sync_attributes"`
}

func (c *Config) validate() error {
	if c.HTTPClientSettings.Endpoint == "" {
		return errors.New("`correlation.endpoint` not specified")
	}

	_, err := url.Parse(c.HTTPClientSettings.Endpoint)
	if err != nil {
		return err
	}

	return nil
}
