// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

const (
	// parserConfigSection is the name that must be used for the parser settings
	// in the configuration struct. The metadata mapstructure for the parser
	// should use the same string.
	parserConfigSection = "parser"
)

var _ confmap.Unmarshaler = (*Config)(nil)

// Config defines configuration for the Carbon receiver.
type Config struct {
	confignet.NetAddr `mapstructure:",squash"`

	// TCPIdleTimeout is the timout for idle TCP connections, it is ignored
	// if transport being used is UDP.
	TCPIdleTimeout time.Duration `mapstructure:"tcp_idle_timeout"`

	// Parser specifies a parser and the respective configuration to be used
	// by the receiver.
	Parser *protocol.Config `mapstructure:"parser"`
}

func (cfg *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		// The section is empty nothing to do, using the default config.
		return nil
	}

	// Unmarshal but not exact yet so the different keys under config do not
	// trigger errors, this is needed so that the types of protocol and transport
	// are read.
	if err := componentParser.Unmarshal(cfg); err != nil {
		return err
	}

	// Unmarshal the protocol, so the type of config can be properly set.
	vParserCfg, errSub := componentParser.Sub(parserConfigSection)
	if errSub != nil {
		return errSub
	}

	if err := protocol.LoadParserConfig(vParserCfg, cfg.Parser); err != nil {
		return fmt.Errorf("error on %q section: %w", parserConfigSection, err)
	}

	// Unmarshal exact to validate the config keys.
	return componentParser.Unmarshal(cfg, confmap.WithErrorUnused())
}
