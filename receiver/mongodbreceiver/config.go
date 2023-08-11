// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	configtls.TLSClientSetting              `mapstructure:"tls,omitempty"`
	// MetricsBuilderConfig defines which metrics/attributes to enable for the scraper
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	Hosts                         []confignet.NetAddr `mapstructure:"hosts"`
	Username                      string              `mapstructure:"username"`
	Password                      configopaque.String `mapstructure:"password"`
	ReplicaSet                    string              `mapstructure:"replica_set,omitempty"`
	Timeout                       time.Duration       `mapstructure:"timeout"`
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
			Password: string(c.Password),
		})
	}

	return clientOptions
}

func (c *Config) hostlist() []string {
	var hosts []string
	for _, ep := range c.Hosts {
		hosts = append(hosts, ep.Endpoint)
	}
	return hosts
}
