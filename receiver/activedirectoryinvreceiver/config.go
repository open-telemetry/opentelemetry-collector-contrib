// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package activedirectoryinvreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectoryinvreceiver"

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

// ADConfig defines configuration for Active Directory Inventory receiver.

type ADConfig struct {
	BaseDN       string        `mapstructure:"base_dn"` // DN is the base distinguished name to search from
	Attributes   []string      `mapstructure:"attributes"`
	PollInterval time.Duration `mapstructure:"poll_interval"`
}

var (
	errEmptyDN             = errors.New("base_dn must not be empty")
	errInvalidDN           = errors.New("base_dn is not a valid distinguished name (expected format: CN=Guest,OU=Users,DC=example,DC=com)")
	errInvalidPollInterval = errors.New("poll_interval is incorrect, invalid duration")
	errSupportedOS         = errors.New("active_directory_inv is only supported on Windows")
)

func isValidDuration(duration time.Duration) bool {
	return duration > 0
}

// Validate validates all portions of the relevant config
func (c *ADConfig) Validate() error {
	// Regular expression pattern for a valid DN
	// CN=Guest,CN=Users,DC=exampledomain,DC=com
	// CN=Guest,OU=Users,DC=exampledomain,DC=com
	// DC=exampledomain,DC=com
	// CN=Guest,DC=exampledomain,DC=com
	// OU=Users,DC=exampledomain,DC=com
	pattern := `^((CN|OU)=[^,]+(,|$))*((DC=[^,]+),?)+$`

	// Compile the regular expression pattern
	regex := regexp.MustCompile(pattern)

	if c.BaseDN == "" {
		return errEmptyDN
	}
	// Check if the Base DN is valid
	if !regex.MatchString(c.BaseDN) {
		return fmt.Errorf("%w: got %q", errInvalidDN, c.BaseDN)
	}

	if !isValidDuration(c.PollInterval) {
		return fmt.Errorf("%w: got %s", errInvalidPollInterval, c.PollInterval)
	}

	return nil
}
