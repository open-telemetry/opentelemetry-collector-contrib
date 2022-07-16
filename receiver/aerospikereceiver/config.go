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
	"time"

	as "github.com/aerospike/aerospike-client-go/v5"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/internal/metadata"
)

var (
	errBadEndpoint          = errors.New("endpoint must include a Host string, Port int, and TLSName string is using TLS")
	errBadPort              = errors.New("invalid port in endpoint")
	errEmptyEndpointHost    = errors.New("endpoint host must be specified")
	errEmptyEndpointPort    = errors.New("endpoint port must be specified")
	errEmptyEndpointTLSName = errors.New("endpoint TLSName must be specified")
	errEmptyPassword        = errors.New("password must be set if username is set")
	errEmptyUsername        = errors.New("username must be set if password is set")
	errNegativeTimeout      = errors.New("timeout must be non-negative")
	errFailedTLSLoad        = errors.New("failed to load TLS config")
)

// Config is the receiver configuration
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	Endpoint                                as.Host                     `mapstructure:"endpoint"`
	Username                                string                      `mapstructure:"username"`
	Password                                string                      `mapstructure:"password"`
	CollectClusterMetrics                   bool                        `mapstructure:"collect_cluster_metrics"`
	Timeout                                 time.Duration               `mapstructure:"timeout"`
	Metrics                                 metadata.MetricsSettings    `mapstructure:"metrics"`
	TLS                                     *configtls.TLSClientSetting `mapstructure:"tls,omitempty"`
}

// Validate validates the values of the given Config, and returns an error if validation fails
func (c *Config) Validate() error {
	var allErrs error

	host := c.Endpoint.Name
	port := c.Endpoint.Port
	TLSName := c.Endpoint.TLSName
	endpointString := fmt.Sprintf("%s:%d", host, port)

	if host == "" {
		return multierr.Append(allErrs, errEmptyEndpointHost)
	}

	if port == 0 {
		return multierr.Append(allErrs, errEmptyEndpointPort)
	}

	_, _, err := net.SplitHostPort(endpointString)
	if err != nil {
		return multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadEndpoint, err))
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

	if c.TLS != nil && TLSName == "" {
		allErrs = multierr.Append(allErrs, fmt.Errorf("%w: when using TLS", errEmptyEndpointTLSName))
	}

	return allErrs
}
