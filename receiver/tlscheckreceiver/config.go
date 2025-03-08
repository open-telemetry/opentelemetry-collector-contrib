// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var errInvalidEndpoint = errors.New(`"endpoint" must be in the form of <hostname>:<port>`)

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Targets                        []*confignet.TCPAddrConfig `mapstructure:"targets"`
}

func validatePort(port string) error {
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("provided port is not a number: %s", port)
	}
	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("provided port is out of valid range (1-65535): %d", portNum)
	}
	return nil
}

func validateTarget(cfg *confignet.TCPAddrConfig) error {
	var err error

	if cfg.Endpoint == "" {
		return errMissingTargets
	}

	if strings.Contains(cfg.Endpoint, "://") {
		return fmt.Errorf("endpoint contains a scheme, which is not allowed: %s", cfg.Endpoint)
	}

	_, port, parseErr := net.SplitHostPort(cfg.Endpoint)
	if parseErr != nil {
		return fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), parseErr)
	}

	portParseErr := validatePort(port)
	if portParseErr != nil {
		return fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), portParseErr)
	}

	return err
}

func (cfg *Config) Validate() error {
	var err error

	if len(cfg.Targets) == 0 {
		err = multierr.Append(err, errMissingTargets)
	}

	for _, target := range cfg.Targets {
		err = multierr.Append(err, validateTarget(target))
	}

	return err
}
