// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package correlation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/correlation"

import (
	"errors"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	conventions "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/correlations"
)

// DefaultConfig returns default configuration correlation values.
func DefaultConfig() *Config {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Timeout = 5 * time.Second
	return &Config{
		ClientConfig:        clientConfig,
		StaleServiceTimeout: 5 * time.Minute,
		SyncAttributes: map[string]string{
			string(conventions.K8SPodUIDKey):   string(conventions.K8SPodUIDKey),
			string(conventions.ContainerIDKey): string(conventions.ContainerIDKey),
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
	confighttp.ClientConfig `mapstructure:",squash"`
	correlations.Config     `mapstructure:",squash"`

	// How long to wait after a trace span's service name is last seen before
	// uncorrelating that service.
	StaleServiceTimeout time.Duration `mapstructure:"stale_service_timeout"`
	// SyncAttributes is a key of the span attribute name to sync to the dimension as the value.
	SyncAttributes map[string]string `mapstructure:"sync_attributes"`
}

func (c *Config) validate() error {
	if c.Endpoint == "" {
		return errors.New("`correlation.endpoint` not specified")
	}

	_, err := url.Parse(c.Endpoint)
	if err != nil {
		return err
	}

	return nil
}
