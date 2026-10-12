// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"encoding/base64"
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

// defaultMaxResponseBytes bounds how much unmatched response data is buffered
// while waiting for an expect step to match.
const defaultMaxResponseBytes = 64 * 1024

// Predefined error responses for configuration validation failures
var (
	errInvalidEndpoint          = errors.New(`"Endpoint" must be in the form of <hostname>:<port>`)
	errMissingTargets           = errors.New(`No targets specified`)
	errConfigTCPCheck           = errors.New(`Invalid Config`)
	errDuplicateTarget          = errors.New(`duplicate target endpoint`)
	errEmptyQueryResponseStep   = errors.New(`at least one of send, send_bytes, expect or expect_bytes must be set`)
	errConflictingSend          = errors.New(`send and send_bytes are mutually exclusive`)
	errConflictingExpect        = errors.New(`expect and expect_bytes are mutually exclusive`)
	errNegativeMaxResponseBytes = errors.New(`max_response_bytes must not be negative`)
)

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	ControllerConfig     scraperhelper.ControllerConfig `mapstructure:",squash"`
	MetricsBuilderConfig metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Targets              []*TargetConfig                `mapstructure:"targets"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// TargetConfig defines a single TCP endpoint to check.
type TargetConfig struct {
	confignet.TCPAddrConfig `mapstructure:",squash"`

	// QueryResponse is an optional ordered list of send/expect steps that are
	// executed on the connection after it is established. When empty, the
	// check only verifies that a TCP connection can be established.
	QueryResponse []QueryResponseStep `mapstructure:"query_response"`

	// MaxResponseBytes bounds the amount of unmatched response data buffered
	// while waiting for an expect step to match. Defaults to 64 KiB.
	MaxResponseBytes int `mapstructure:"max_response_bytes"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// QueryResponseStep is a single step of a query/response exchange. When a step
// specifies both an expect and a send operation, the expect operation runs first.
type QueryResponseStep struct {
	// Send is a string written to the connection. When the same step also
	// specifies Expect, capture groups of the match can be referenced as
	// ${1}, ${2}, ... and a literal $ must be written as $$.
	Send string `mapstructure:"send"`
	// SendBytes is base64-encoded data written to the connection as-is.
	// Mutually exclusive with Send.
	SendBytes string `mapstructure:"send_bytes"`
	// Expect is a regular expression. Data is read from the connection until
	// it matches.
	Expect string `mapstructure:"expect"`
	// ExpectBytes is a base64-encoded byte sequence. Data is read from the
	// connection until it contains the sequence. Mutually exclusive with Expect.
	ExpectBytes string `mapstructure:"expect_bytes"`

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

func validateTarget(cfg *TargetConfig) error {
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

	if cfg.MaxResponseBytes < 0 {
		err = multierr.Append(err, errNegativeMaxResponseBytes)
	}

	maxResponseBytes := cfg.MaxResponseBytes
	if maxResponseBytes <= 0 {
		maxResponseBytes = defaultMaxResponseBytes
	}
	for i := range cfg.QueryResponse {
		step, stepErr := cfg.QueryResponse[i].compile()
		if stepErr != nil {
			err = multierr.Append(err, fmt.Errorf("query_response step %d: %w", i, stepErr))
			continue
		}
		if len(step.expectBytes) > maxResponseBytes {
			err = multierr.Append(err, fmt.Errorf("query_response step %d: expect_bytes is longer than max_response_bytes (%d) and can never match", i, maxResponseBytes))
		}
	}

	return err
}

func (cfg *Config) Validate() error {
	var err error

	if len(cfg.Targets) == 0 {
		err = multierr.Append(err, errMissingTargets)
	}

	seen := make(map[string]struct{}, len(cfg.Targets))
	for _, tcpConfig := range cfg.Targets {
		err = multierr.Append(err, validateTarget(tcpConfig))
		if _, ok := seen[tcpConfig.Endpoint]; ok {
			err = multierr.Append(err, fmt.Errorf("%w: %s", errDuplicateTarget, tcpConfig.Endpoint))
			continue
		}
		seen[tcpConfig.Endpoint] = struct{}{}
	}

	return err
}

// compiledStep is a QueryResponseStep with its pattern and payloads decoded,
// so that no parsing happens on every check.
type compiledStep struct {
	send        []byte
	sendIsTmpl  bool
	expectRe    *regexp.Regexp
	expectBytes []byte
}

func (s *QueryResponseStep) compile() (compiledStep, error) {
	var cs compiledStep
	var err error

	if s.Send == "" && s.SendBytes == "" && s.Expect == "" && s.ExpectBytes == "" {
		return cs, errEmptyQueryResponseStep
	}
	if s.Send != "" && s.SendBytes != "" {
		err = multierr.Append(err, errConflictingSend)
	}
	if s.Expect != "" && s.ExpectBytes != "" {
		err = multierr.Append(err, errConflictingExpect)
	}

	if s.Expect != "" {
		re, reErr := regexp.Compile(s.Expect)
		if reErr != nil {
			err = multierr.Append(err, fmt.Errorf("invalid expect regular expression: %w", reErr))
		}
		cs.expectRe = re
	}
	if s.ExpectBytes != "" {
		b, decErr := base64.StdEncoding.DecodeString(s.ExpectBytes)
		switch {
		case decErr != nil:
			err = multierr.Append(err, fmt.Errorf("invalid expect_bytes: %w", decErr))
		case len(b) == 0:
			err = multierr.Append(err, errEmptyBytes("expect_bytes"))
		}
		cs.expectBytes = b
	}
	if s.Send != "" {
		cs.send = []byte(s.Send)
		// Only a send that follows an expect in the same step can reference
		// capture groups; a standalone send is written literally.
		cs.sendIsTmpl = cs.expectRe != nil
	}
	if s.SendBytes != "" {
		b, decErr := base64.StdEncoding.DecodeString(s.SendBytes)
		switch {
		case decErr != nil:
			err = multierr.Append(err, fmt.Errorf("invalid send_bytes: %w", decErr))
		case len(b) == 0:
			err = multierr.Append(err, errEmptyBytes("send_bytes"))
		}
		cs.send = b
	}

	return cs, err
}

// errEmptyBytes reports a base64 field that decodes to nothing, which the
// decoder otherwise accepts silently (e.g. a value of only whitespace).
func errEmptyBytes(field string) error {
	return fmt.Errorf("%s decodes to zero bytes", field)
}
