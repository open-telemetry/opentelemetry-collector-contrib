// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package wavefrontreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/confignet"
)

// Config defines configuration for the Wavefront receiver.
type Config struct {
	confignet.TCPAddr `mapstructure:",squash"`

	// TCPIdleTimeout is the timout for idle TCP connections.
	TCPIdleTimeout time.Duration `mapstructure:"tcp_idle_timeout"`

	// ExtractCollectdTags instructs the Wavefront receiver to attempt to extract
	// tags in the CollectD format from the metric name. The default is false.
	ExtractCollectdTags bool `mapstructure:"extract_collectd_tags"`
}

func (cfg *Config) Validate() error {
	if cfg.TCPIdleTimeout < 0 {
		return errors.New("'tcp_idle_timeout' must be non-negative")
	}
	return nil
}
