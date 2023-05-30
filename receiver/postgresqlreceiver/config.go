// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"errors"
	"fmt"
	"net"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

// Errors for missing required config parameters.
const (
	ErrNoUsername          = "invalid config: missing username"
	ErrNoPassword          = "invalid config: missing password" // #nosec G101 - not hardcoded credentials
	ErrNotSupported        = "invalid config: field '%s' not supported"
	ErrTransportsSupported = "invalid config: 'transport' must be 'tcp' or 'unix'"
	ErrHostPort            = "invalid config: 'endpoint' must be in the form <host>:<port> no matter what 'transport' is configured"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	Username                                string                         `mapstructure:"username"`
	Password                                string                         `mapstructure:"password"`
	Databases                               []string                       `mapstructure:"databases"`
	confignet.NetAddr                       `mapstructure:",squash"`       // provides Endpoint and Transport
	configtls.TLSClientSetting              `mapstructure:"tls,omitempty"` // provides SSL details
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
}

func (cfg *Config) Validate() error {
	var err error
	if cfg.Username == "" {
		err = multierr.Append(err, errors.New(ErrNoUsername))
	}
	if cfg.Password == "" {
		err = multierr.Append(err, errors.New(ErrNoPassword))
	}

	// The lib/pq module does not support overriding ServerName or specifying supported TLS versions
	if cfg.ServerName != "" {
		err = multierr.Append(err, fmt.Errorf(ErrNotSupported, "ServerName"))
	}
	if cfg.MaxVersion != "" {
		err = multierr.Append(err, fmt.Errorf(ErrNotSupported, "MaxVersion"))
	}
	if cfg.MinVersion != "" {
		err = multierr.Append(err, fmt.Errorf(ErrNotSupported, "MinVersion"))
	}

	switch cfg.Transport {
	case "tcp", "unix":
		_, _, endpointErr := net.SplitHostPort(cfg.Endpoint)
		if endpointErr != nil {
			err = multierr.Append(err, errors.New(ErrHostPort))
		}
	default:
		err = multierr.Append(err, errors.New(ErrTransportsSupported))
	}

	return err
}
