// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package simpleprometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver"

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	configutil "github.com/prometheus/common/config"
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config defines configuration for simple prometheus receiver.
type Config struct {
	ClientConfig confighttp.ClientConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	// CollectionInterval is the interval at which metrics should be collected
	CollectionInterval time.Duration `mapstructure:"collection_interval"`
	// MetricsPath the path to the metrics endpoint.
	MetricsPath string `mapstructure:"metrics_path"`
	// Params the parameters to the metrics endpoint.
	Params url.Values `mapstructure:"params,omitempty"`
	// Labels static labels
	Labels map[string]string `mapstructure:"labels,omitempty"`
	// Whether or not to use pod service account to authenticate.
	UseServiceAccount bool `mapstructure:"use_service_account"`
	// JobName allows users to customize the job name optionally.
	JobName string `mapstructure:"job_name"`
}

// Validate checks the receiver configuration is valid.
func (cfg *Config) Validate() error {
	if cfg.ClientConfig.Endpoint == "" {
		return errors.New("endpoint must be specified")
	}
	if cfg.CollectionInterval <= 0 {
		return fmt.Errorf("collection_interval must be positive, got %s", cfg.CollectionInterval)
	}
	for _, header := range cfg.ClientConfig.Headers {
		if _, ok := configutil.ReservedHeaders[http.CanonicalHeaderKey(header.Name)]; ok {
			return fmt.Errorf("headers: header %q is reserved and cannot be set", header.Name)
		}
	}
	if cfg.ClientConfig.ProxyURL != "" {
		proxyURL, err := url.ParseRequestURI(cfg.ClientConfig.ProxyURL)
		if err != nil {
			return fmt.Errorf("proxy_url is not valid: %w", err)
		}
		if proxyURL.User != nil {
			return errors.New("proxy_url must not contain credentials; see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/52081")
		}
	}
	return nil
}
