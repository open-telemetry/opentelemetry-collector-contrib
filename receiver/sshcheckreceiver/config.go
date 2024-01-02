// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sshcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver"

import (
	"errors"
	"net"
	"strings"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver/internal/configssh"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver/internal/metadata"
)

// Predefined error responses for configuration validation failures
var (
	errMissingEndpoint           = errors.New(`"endpoint" not specified in config`)
	errInvalidEndpoint           = errors.New(`"endpoint" is invalid`)
	errMissingUsername           = errors.New(`"username" not specified in config`)
	errMissingPasswordAndKeyFile = errors.New(`either "password" or "key_file" is required`)

	errConfigNotSSHCheck  = errors.New("config was not a SSH check receiver config")
	errWindowsUnsupported = errors.New(metadata.Type + " is unsupported on Windows.")
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	configssh.SSHClientSettings             `mapstructure:",squash"`

	CheckSFTP            bool                          `mapstructure:"check_sftp"`
	MetricsBuilderConfig metadata.MetricsBuilderConfig `mapstructure:",squash"`
}

// SFTPEnabled tells whether SFTP metrics are Enabled in MetricsSettings.
func (c Config) SFTPEnabled() bool {
	return (c.CheckSFTP || c.MetricsBuilderConfig.Metrics.SshcheckSftpDuration.Enabled || c.MetricsBuilderConfig.Metrics.SshcheckSftpStatus.Enabled)
}

func (c Config) Validate() (err error) {
	if c.SSHClientSettings.Endpoint == "" {
		err = multierr.Append(err, errMissingEndpoint)
	} else if strings.Contains(c.SSHClientSettings.Endpoint, " ") {
		err = multierr.Append(err, errInvalidEndpoint)
	} else if _, _, splitErr := net.SplitHostPort(c.SSHClientSettings.Endpoint); splitErr != nil {
		err = multierr.Append(splitErr, errInvalidEndpoint)
	}

	if c.SSHClientSettings.Username == "" {
		err = multierr.Append(err, errMissingUsername)
	}

	if c.SSHClientSettings.Password == "" && c.KeyFile == "" {
		err = multierr.Append(err, errMissingPasswordAndKeyFile)
	}

	return
}
