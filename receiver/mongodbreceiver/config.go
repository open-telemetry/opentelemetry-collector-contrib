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

	// Hosts are a list of <host>:<port> or unix domain sockets
	// for standalone deployments, use the hostname and port of the mongod instance
	// for sharded deployments it is the list of mongos instances
	// for replica set deployments
	Hosts []confignet.NetAddr `mapstructure:"hosts"`
	// Username to use for authentication; optional
	Username string `mapstructure:"username"`
	// Password to use for authentication; required if Username is present
	Password string `mapstructure:"password"`
	// ReplicaSet if supplied will allow autodiscovery of
	// any replica set members when given the address of any one valid member; optional
	ReplicaSet string `mapstructure:"replica_set,omitempty"`

	Timeout time.Duration `mapstructure:"timeout"`
}

func (c *Config) Validate() error {
	if len(c.Hosts) == 0 {
		return errors.New("no hosts were specified in the config")
	}

	var err error
	for _, host := range c.Hosts {
		isHostnameErr := isHostnamePort(host.Endpoint)
		isHostname := isHostnameErr != nil
		if !isHostname && host.Transport != "unix" {
			err = multierr.Append(err, fmt.Errorf("endpoint '%s' does not match format of '<host>:<port>': %w", host.Endpoint, isHostnameErr))
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

func isHostnamePort(s string) error {
	_, _, err := net.SplitHostPort(s)
	if err != nil {
		return err
	}
	return nil
}
