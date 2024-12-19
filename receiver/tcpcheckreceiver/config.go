// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"errors"
	"net"
	"strings"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/configtcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var (
	errMissingEndpoint   = errors.New(`"endpoint" not specified in config`)
	errInvalidEndpoint   = errors.New(`"endpoint" is invalid`)
	errConfigNotTCPCheck = errors.New("config was not a TCP check receiver config")
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	configtcp.TCPClientSettings    `mapstructure:",squash"`
	MetricsBuilderConfig           metadata.MetricsBuilderConfig `mapstructure:",squash"`
}

func (c Config) Validate() (err error) {
	if c.TCPClientSettings.Endpoint == "" {
		err = multierr.Append(err, errMissingEndpoint)
	} else if strings.Contains(c.TCPClientSettings.Endpoint, " ") {
		err = multierr.Append(err, errInvalidEndpoint)
	} else if _, _, splitErr := net.SplitHostPort(c.TCPClientSettings.Endpoint); splitErr != nil {
		err = multierr.Append(splitErr, errInvalidEndpoint)
	}
	return
}
