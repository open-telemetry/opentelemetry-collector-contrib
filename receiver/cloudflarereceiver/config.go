// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"errors"
	"fmt"
	"net"

	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/multierr"
)

// Config holds all the parameters to start an HTTP server that can be sent logs from CloudFlare
type Config struct {
	Logs LogsConfig `mapstructure:"logs"`
}

type LogsConfig struct {
	Secret         string                      `mapstructure:"secret"`
	Endpoint       string                      `mapstructure:"endpoint"`
	TLS            *configtls.TLSServerSetting `mapstructure:"tls"`
	Attributes     map[string]string           `mapstructure:"attributes"`
	TimestampField string                      `mapstructure:"timestamp_field"`
}

var (
	errNoEndpoint = errors.New("an endpoint must be specified")
	errNoTLS      = errors.New("tls must be configured")
	errNoCert     = errors.New("tls was configured, but no cert file was specified")
	errNoKey      = errors.New("tls was configured, but no key file was specified")

	defaultTimestampField = "EdgeStartTimestamp"
)

func (c *Config) Validate() error {
	if c.Logs.Endpoint == "" {
		return errNoEndpoint
	}

	if c.Logs.TLS == nil {
		return errNoTLS
	}

	var errs error
	_, _, err := net.SplitHostPort(c.Logs.Endpoint)
	if err != nil {
		errs = multierr.Append(errs, fmt.Errorf("failed to split endpoint into 'host:port' pair: %w", err))
	}

	if c.Logs.TLS.CertFile == "" {
		errs = multierr.Append(errs, errNoCert)
	}

	if c.Logs.TLS.KeyFile == "" {
		errs = multierr.Append(errs, errNoKey)
	}

	return errs
}
