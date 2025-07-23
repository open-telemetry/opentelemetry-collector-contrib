// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var (
	errInvalidEndpoint = errors.New(`"Endpoint" must be in the form of <hostname>:<port>`)
	errMissingTargets  = errors.New(`No targets specified`)
	errConfigTCPCheck  = errors.New(`Invalid Config`)
)

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Targets                        []*confignet.TCPAddrConfig `mapstructure:"targets"`

	// prevent unkeyed literal initialization
	_ struct{}
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

	for _, tcpConfig := range cfg.Targets {
		err = multierr.Append(err, validateTarget(tcpConfig))
	}

	return err
}
