// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver"

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
)

const (
	// Default configuration values
	defaultMaxOpenConnections = 5  // Standard default for Oracle connections
	defaultCollectionInterval = 10 * time.Second

	// Validation ranges
	minCollectionInterval = 10 * time.Second
	maxCollectionInterval = 3600 * time.Second
	minMaxOpenConnections = 1
	maxMaxOpenConnections = 1000
	maxUsernameLength     = 128
	maxServiceLength      = 128
)

var (
	errBadDataSource       = errors.New("datasource is invalid")
	errBadEndpoint         = errors.New("endpoint must be specified as host:port")
	errBadPort             = errors.New("invalid port in endpoint")
	errEmptyEndpoint       = errors.New("endpoint must be specified")
	errEmptyPassword       = errors.New("password must be set")
	errEmptyService        = errors.New("service must be specified")
	errEmptyUsername       = errors.New("username must be set")
	errInvalidConnections  = errors.New("max_open_connections must be between 1 and 1000")
	errInvalidTimeout      = errors.New("collection_interval must be between 10s and 3600s")
	errInvalidUsername     = errors.New("username cannot contain special characters that could cause SQL injection")
	errInvalidService      = errors.New("service name cannot contain special characters")
)

type Config struct {
	DataSource string `mapstructure:"datasource"`
	Endpoint   string `mapstructure:"endpoint"`
	Password   string `mapstructure:"password"`
	Service    string `mapstructure:"service"`
	Username   string `mapstructure:"username"`

	// Connection Pool Configuration
	MaxOpenConnections    int  `mapstructure:"max_open_connections"`
	DisableConnectionPool bool `mapstructure:"disable_connection_pool"`

	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
}

// SetDefaults sets default values for configuration fields that are not explicitly set
func (c *Config) SetDefaults() {
	if c.MaxOpenConnections == 0 {
		c.MaxOpenConnections = defaultMaxOpenConnections
	}
	
	// Set scraper controller defaults if not set
	if c.ControllerConfig.CollectionInterval == 0 {
		c.ControllerConfig.CollectionInterval = defaultCollectionInterval
	}
}

func (c Config) Validate() error {
	var allErrs error

	// If DataSource is defined it takes precedence over the rest of the connection options.
	if c.DataSource == "" {
		// Validate endpoint
		if c.Endpoint == "" {
			allErrs = multierr.Append(allErrs, errEmptyEndpoint)
		} else {
			if err := c.validateEndpoint(); err != nil {
				allErrs = multierr.Append(allErrs, err)
			}
		}

		// Validate username
		if c.Username == "" {
			allErrs = multierr.Append(allErrs, errEmptyUsername)
		} else {
			if err := c.validateUsername(); err != nil {
				allErrs = multierr.Append(allErrs, err)
			}
		}

		// Validate password
		if c.Password == "" {
			allErrs = multierr.Append(allErrs, errEmptyPassword)
		}

		// Validate service
		if c.Service == "" {
			allErrs = multierr.Append(allErrs, errEmptyService)
		} else {
			if err := c.validateService(); err != nil {
				allErrs = multierr.Append(allErrs, err)
			}
		}
	} else {
		if err := c.validateDataSource(); err != nil {
			allErrs = multierr.Append(allErrs, err)
		}
	}

	// Validate connection pool configuration
	if err := c.validateConnectionPool(); err != nil {
		allErrs = multierr.Append(allErrs, err)
	}

	// Validate scraper controller configuration
	if err := c.validateScraperConfig(); err != nil {
		allErrs = multierr.Append(allErrs, err)
	}

	return allErrs
}

// validateEndpoint validates the database endpoint format and values
func (c Config) validateEndpoint() error {
	host, portStr, err := net.SplitHostPort(c.Endpoint)
	if err != nil {
		return fmt.Errorf("%w: %s", errBadEndpoint, err.Error())
	}

	if host == "" {
		return errBadEndpoint
	}

	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		return fmt.Errorf("%w: %s", errBadPort, err.Error())
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("%w: %d (must be between 1 and 65535)", errBadPort, port)
	}

	return nil
}

// validateUsername validates the username for security and length constraints
func (c Config) validateUsername() error {
	if len(c.Username) > maxUsernameLength {
		return fmt.Errorf("%w: length %d exceeds maximum %d", errInvalidUsername, len(c.Username), maxUsernameLength)
	}

	// Check for potentially dangerous characters that could indicate SQL injection attempts
	// Note: In character classes, - should be escaped or placed at start/end to avoid range interpretation
	dangerousChars := regexp.MustCompile(`[';"\\/\*-]`)
	if dangerousChars.MatchString(c.Username) {
		return errInvalidUsername
	}

	return nil
}

// validateService validates the Oracle service name
func (c Config) validateService() error {
	if len(c.Service) > maxServiceLength {
		return fmt.Errorf("%w: length %d exceeds maximum %d", errInvalidService, len(c.Service), maxServiceLength)
	}

	// Oracle service names should not contain certain special characters
	if strings.ContainsAny(c.Service, " \t\n\r;\"'\\") {
		return errInvalidService
	}

	return nil
}

// validateDataSource validates the complete data source URL
func (c Config) validateDataSource() error {
	parsedURL, err := url.Parse(c.DataSource)
	if err != nil {
		return fmt.Errorf("%w: %s", errBadDataSource, err.Error())
	}

	// Validate scheme
	if parsedURL.Scheme != "oracle" {
		return fmt.Errorf("%w: scheme must be 'oracle', got '%s'", errBadDataSource, parsedURL.Scheme)
	}

	// Validate host and port are present
	if parsedURL.Host == "" {
		return fmt.Errorf("%w: host must be specified", errBadDataSource)
	}

	return nil
}

// validateConnectionPool validates connection pool settings
func (c Config) validateConnectionPool() error {
	if c.MaxOpenConnections < minMaxOpenConnections || c.MaxOpenConnections > maxMaxOpenConnections {
		return fmt.Errorf("%w: got %d", errInvalidConnections, c.MaxOpenConnections)
	}

	return nil
}

// validateScraperConfig validates scraper controller configuration
func (c Config) validateScraperConfig() error {
	if c.ControllerConfig.CollectionInterval < minCollectionInterval || c.ControllerConfig.CollectionInterval > maxCollectionInterval {
		return fmt.Errorf("%w: got %v", errInvalidTimeout, c.ControllerConfig.CollectionInterval)
	}

	return nil
}
