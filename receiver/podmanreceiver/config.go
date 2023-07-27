// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

var _ component.Config = (*Config)(nil)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`

	// The URL of the podman server.  Default is "unix:///run/podman/podman.sock"
	Endpoint string `mapstructure:"endpoint"`

	APIVersion    string              `mapstructure:"api_version"`
	SSHKey        string              `mapstructure:"ssh_key"`
	SSHPassphrase configopaque.String `mapstructure:"ssh_passphrase"`
}

func (config Config) Validate() error {
	if config.Endpoint == "" {
		return errors.New("config.Endpoint must be specified")
	}
	return nil
}
