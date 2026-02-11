// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbon // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver/internal/carbon"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver/internal/protocol"
)

var _ xconfmap.Validator = (*Config)(nil)

// Config defines configuration for the Carbon receiver.
type Config struct {
	confignet.AddrConfig `mapstructure:",squash"`

	// TCPIdleTimeout is the timeout for idle TCP connections, it is ignored
	// if transport being used is UDP.
	TCPIdleTimeout time.Duration `mapstructure:"tcp_idle_timeout"`

	// Parser specifies a parser and the respective configuration to be used
	// by the receiver.
	Parser *protocol.Config `mapstructure:"parser"`
}

func (cfg *Config) Validate() error {
	if cfg.TCPIdleTimeout < 0 {
		return errors.New("'tcp_idle_timeout' must be non-negative")
	}
	return nil
}
