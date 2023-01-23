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

package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/internal/metadata"
)

var (
	errBadEndpoint          = errors.New("endpoint must be specified as host:port")
	errBadPort              = errors.New("invalid port in endpoint")
	errEmptyEndpoint        = errors.New("endpoint must be specified")
	errEmptyEndpointTLSName = errors.New("endpoint TLSName must be specified")
	errEmptyPassword        = errors.New("password must be set if username is set")
	errEmptyUsername        = errors.New("username must be set if password is set")
	errNegativeTimeout      = errors.New("timeout must be non-negative")
	errFailedTLSLoad        = errors.New("failed to load TLS config")
)

// Config is the receiver configuration
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	Endpoint                                string        `mapstructure:"endpoint"`
	TLSName                                 string        `mapstructure:"tlsname"`
	Username                                string        `mapstructure:"username"`
	Password                                string        `mapstructure:"password"`
	CollectClusterMetrics                   bool          `mapstructure:"collect_cluster_metrics"`
	Timeout                                 time.Duration `mapstructure:"timeout"`
	metadata.MetricsBuilderConfig           `mapstructure:",squash"`
	TLS                                     *configtls.TLSClientSetting `mapstructure:"tls,omitempty"`
}

// Validate validates the values of the given Config, and returns an error if validation fails
func (c *Config) Validate() error {
	var allErrs error

	if c.Endpoint == "" {
		return multierr.Append(allErrs, errEmptyEndpoint)
	}

	host, portStr, err := net.SplitHostPort(c.Endpoint)
	if err != nil {
		return multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadEndpoint, err))
	}

	if host == "" {
		allErrs = multierr.Append(allErrs, errBadEndpoint)
	}

	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadPort, err))
	}

	if port < 0 || port > 65535 {
		allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %d", errBadPort, port))
	}

	if c.Username != "" && c.Password == "" {
		allErrs = multierr.Append(allErrs, errEmptyPassword)
	}

	if c.Password != "" && c.Username == "" {
		allErrs = multierr.Append(allErrs, errEmptyUsername)
	}
	if c.Timeout.Milliseconds() < 0 {
		allErrs = multierr.Append(allErrs, fmt.Errorf("%w: must be positive", errNegativeTimeout))
	}

	if c.TLS != nil {
		_, err := c.TLS.LoadTLSConfig()
		if err != nil {
			allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %s", errFailedTLSLoad, err))
		}
	}

	if c.TLS != nil && c.TLSName == "" {
		allErrs = multierr.Append(allErrs, fmt.Errorf("%w: when using TLS", errEmptyEndpointTLSName))
	}

	return allErrs
}
