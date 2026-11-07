// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"errors"
	"fmt"
	"net"
	"regexp"
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
	errDuplicateTarget = errors.New(`duplicate target endpoint`)
)

const defaultMaxReadSize = 64 * 1024

// TargetConfig defines a TCP endpoint and optional application-level checks.
type TargetConfig struct {
	confignet.TCPAddrConfig `mapstructure:",squash"`
	// QueryResponse is an ordered sequence of expect and send operations.
	QueryResponse []QueryResponse `mapstructure:"query_response"`
	// MaxReadSize limits the buffered response bytes per expect operation. Zero uses 64 KiB.
	MaxReadSize int `mapstructure:"max_read_size"`
}

// QueryResponse defines a step that expects a response before sending data.
type QueryResponse struct {
	// Send is written verbatim, with capture references expanded from this step's Expect.
	Send string `mapstructure:"send"`
	// SendBytes contains base64-encoded bytes and is mutually exclusive with Send.
	SendBytes string `mapstructure:"send_bytes"`
	// Expect is a regular expression to match in the response stream.
	Expect string `mapstructure:"expect"`
	// ExpectBytes contains base64-encoded bytes to find in the response stream.
	ExpectBytes string `mapstructure:"expect_bytes"`

	expectRegexp *regexp.Regexp
	sendData     []byte
	expectData   []byte
}

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	ControllerConfig     scraperhelper.ControllerConfig `mapstructure:",squash"`
	MetricsBuilderConfig metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Targets              []*TargetConfig                `mapstructure:"targets"`

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

	seen := make(map[string]struct{}, len(cfg.Targets))
	for i, tcpConfig := range cfg.Targets {
		if tcpConfig == nil {
			err = multierr.Append(err, fmt.Errorf("targets[%d]: target must not be null", i))
			continue
		}
		err = multierr.Append(err, validateTarget(&tcpConfig.TCPAddrConfig))
		if queryErr := tcpConfig.validateQueryResponse(); queryErr != nil {
			err = multierr.Append(err, fmt.Errorf("targets[%d]: %w", i, queryErr))
		}
		if _, ok := seen[tcpConfig.Endpoint]; ok {
			err = multierr.Append(err, fmt.Errorf("%w: %s", errDuplicateTarget, tcpConfig.Endpoint))
			continue
		}
		seen[tcpConfig.Endpoint] = struct{}{}
	}

	return err
}
