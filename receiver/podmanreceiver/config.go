// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

var _ component.Config = (*Config)(nil)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`

	// The URL of the podman server.  Default is "unix:///run/podman/podman.sock"
	Endpoint string `mapstructure:"endpoint"`

	// The maximum amount of time to wait for Podman API responses.  Default is 5s
	Timeout time.Duration `mapstructure:"timeout"`

	APIVersion    string `mapstructure:"api_version"`
	SSHKey        string `mapstructure:"ssh_key"`
	SSHPassphrase string `mapstructure:"ssh_passphrase"`
}

func (config Config) Validate() error {
	if config.Endpoint == "" {
		return errors.New("config.Endpoint must be specified")
	}
	if config.CollectionInterval == 0 {
		return errors.New("config.CollectionInterval must be specified")
	}
	return nil
}
