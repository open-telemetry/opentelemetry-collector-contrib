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

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	configtls.TLSClientSetting              `mapstructure:"tls,omitempty"`
	// Metrics defines which metrics to enable for the scraper
	Metrics    metadata.MetricsSettings `mapstructure:"metrics"`
	Hosts      []confignet.NetAddr      `mapstructure:"hosts"`
	Username   string                   `mapstructure:"username"`
	Password   string                   `mapstructure:"password"`
	ReplicaSet string                   `mapstructure:"replica_set,omitempty"`
	Timeout    time.Duration            `mapstructure:"timeout"`
}

func (c *Config) Validate() error {
	if len(c.Hosts) == 0 {
		return errors.New("no hosts were specified in the config")
	}

	var err error
	for _, host := range c.Hosts {
		if host.Endpoint == "" {
			err = multierr.Append(err, errors.New("no endpoint specified for one of the hosts"))
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

func (c *Config) ClientOptions() *options.ClientOptions {
	clientOptions := options.Client()
	connString := fmt.Sprintf("mongodb://%s", strings.Join(c.hostlist(), ","))
	clientOptions.ApplyURI(connString)

	if c.Timeout > 0 {
		clientOptions.SetConnectTimeout(c.Timeout)
	}

	tlsConfig, err := c.LoadTLSConfig()
	if err == nil && tlsConfig != nil {
		clientOptions.SetTLSConfig(tlsConfig)
	}

	if c.ReplicaSet != "" {
		clientOptions.SetReplicaSet(c.ReplicaSet)
	}

	if c.Username != "" && c.Password != "" {
		clientOptions.SetAuth(options.Credential{
			Username: c.Username,
			Password: c.Password,
		})
	}

	return clientOptions
}

func (c *Config) hostlist() []string {
	hosts := []string{}
	for _, ep := range c.Hosts {
		hosts = append(hosts, ep.Endpoint)
	}
	return hosts
}
