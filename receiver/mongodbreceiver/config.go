// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongodbreceiver

import (
	"errors"
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	configtls.TLSClientSetting              `mapstructure:"tls,omitempty"`
	Hosts                                   []confignet.NetAddr `mapstructure:"hosts"`
	Username                                string              `mapstructure:"username"`
	Password                                string              `mapstructure:"password"`
	ReplicaSet                              string              `mapstructure:"replica_set,omitempty"`
	Timeout                                 time.Duration       `mapstructure:"timeout"`
}

func (c *Config) Validate() error {
	if len(c.Hosts) == 0 {
		return errors.New("no hosts were specified in the config")
	}

	var err error
	for _, host := range c.Hosts {
		if host.Transport != "unix" {
			// valid hostnames will use default port of 27017
			_, hostNameErr := net.LookupHost(host.Endpoint)
			if hostNameErr == nil {
				continue
			}
			// check host:port based endpoints
			_, _, hpErr := net.SplitHostPort(host.Endpoint)
			if hpErr != nil {
				err = multierr.Append(err, fmt.Errorf("unknown host format for host %s: %w", host.Endpoint, hpErr))
			}
		}
	}

	if c.Username != "" && c.Password == "" {
		err = multierr.Append(err, errors.New("username provided without password"))
	} else if c.Username == "" && c.Password != "" {
		err = multierr.Append(err, errors.New("password provided without user"))
	}

	if _, tlsErr := c.LoadTLSConfig(); tlsErr != nil {
		err = multierr.Append(err, fmt.Errorf("error loading tls configuration: %w", tlsErr))
	}

	return err
}
