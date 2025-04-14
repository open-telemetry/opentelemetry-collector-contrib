// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var errInvalidEndpoint = errors.New(`"endpoint" must be in the form of <hostname>:<port>`)

// CertificateTarget represents a target for certificate checking, which can be either
// a network endpoint or a local file
type CertificateTarget struct {
	confignet.TCPAddrConfig `mapstructure:",squash"`
	FilePath                string `mapstructure:"file_path"`
}

// Config defines the configuration for the various elements of the receiver agent.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Targets                        []*CertificateTarget `mapstructure:"targets"`
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

func validateTarget(ct *CertificateTarget) error {
	// Check that exactly one of endpoint or file_path is specified
	if ct.Endpoint != "" && ct.FilePath != "" {
		return errors.New("cannot specify both endpoint and file_path")
	}
	if ct.Endpoint == "" && ct.FilePath == "" {
		return errors.New("must specify either endpoint or file_path")
	}

	// Validate endpoint if specified
	if ct.Endpoint != "" {
		if strings.Contains(ct.Endpoint, "://") {
			return fmt.Errorf("endpoint contains a scheme, which is not allowed: %s", ct.Endpoint)
		}

		_, port, err := net.SplitHostPort(ct.Endpoint)
		if err != nil {
			return fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), err)
		}

		if err := validatePort(port); err != nil {
			return fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), err)
		}
	}

	// Validate file path if specified
	if ct.FilePath != "" {
		// Clean the path to handle different path separators
		cleanPath := filepath.Clean(ct.FilePath)

		// Check if the path is absolute
		if !filepath.IsAbs(cleanPath) {
			return fmt.Errorf("file path must be absolute: %s", ct.FilePath)
		}

		// Check if path exists and is a regular file
		fileInfo, err := os.Stat(cleanPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("certificate file does not exist: %s", ct.FilePath)
			}
			return fmt.Errorf("error accessing certificate file %s: %w", ct.FilePath, err)
		}

		// check if it is a directory
		if fileInfo.IsDir() {
			return fmt.Errorf("path is a directory, not a file: %s", cleanPath)
		}

		// Check if it's a regular file (not a directory or special file)
		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("certificate path is not a regular file: %s", ct.FilePath)
		}

		// Check if file is readable
		if _, err := os.ReadFile(cleanPath); err != nil {
			return fmt.Errorf("certificate file is not readable: %s", ct.FilePath)
		}
	}

	return nil
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
