// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	configtls.ClientConfig         `mapstructure:"tls,omitempty"`
	// MetricsBuilderConfig defines which metrics/attributes to enable for the scraper
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	// Deprecated - Transport option will be removed in v0.102.0
	Hosts                   []confignet.TCPAddrConfig `mapstructure:"hosts"`
	Username                string                    `mapstructure:"username"`
	Password                configopaque.String       `mapstructure:"password"`
	AuthMechanism           string                    `mapstructure:"auth_mechanism,omitempty"`
	AuthSource              string                    `mapstructure:"auth_source,omitempty"`
	AuthMechanismProperties map[string]string         `mapstructure:"auth_mechanism_properties,omitempty"`
	ReplicaSet              string                    `mapstructure:"replica_set,omitempty"`
	Timeout                 time.Duration             `mapstructure:"timeout"`
	DirectConnection        bool                      `mapstructure:"direct_connection"`
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

	if _, tlsErr := c.LoadTLSConfig(context.Background()); tlsErr != nil {
		err = multierr.Append(err, fmt.Errorf("error loading tls configuration: %w", tlsErr))
	}

	return err
}

func (c *Config) ClientOptions(secondary bool) *options.ClientOptions {
	if secondary {
		// For secondary nodes, create a direct connection
		clientOptions := options.Client().
			SetHosts(c.hostlist()).
			SetDirect(true).
			SetReadPreference(readpref.SecondaryPreferred())

		if c.Timeout > 0 {
			clientOptions.SetConnectTimeout(c.Timeout)
		}

		// Set up authentication if username/password are provided or if an auth mechanism is specified
		// Some mechanisms (e.g., MONGODB-X509, MONGODB-AWS with IAM) don't require username/password
		if c.Username != "" && c.Password != "" || c.AuthMechanism != "" {
			credential := c.buildCredential()
			clientOptions.SetAuth(credential)
		}

		return clientOptions
	}
	clientOptions := options.Client()
	connString := "mongodb://" + strings.Join(c.hostlist(), ",")
	clientOptions.ApplyURI(connString)

	if c.Timeout > 0 {
		clientOptions.SetConnectTimeout(c.Timeout)
	}

	tlsConfig, err := c.LoadTLSConfig(context.Background())
	if err == nil && tlsConfig != nil {
		clientOptions.SetTLSConfig(tlsConfig)
	}

	if c.ReplicaSet != "" {
		clientOptions.SetReplicaSet(c.ReplicaSet)
	}

	if c.DirectConnection {
		clientOptions.SetDirect(c.DirectConnection)
	}

	// Set up authentication if username/password are provided or if an auth mechanism is specified
	// Some mechanisms (e.g., MONGODB-X509, MONGODB-AWS with IAM) don't require username/password
	if c.Username != "" && c.Password != "" || c.AuthMechanism != "" {
		credential := c.buildCredential()
		clientOptions.SetAuth(credential)
	}

	return clientOptions
}

func (c *Config) buildCredential() options.Credential {
	credential := options.Credential{}
	if c.Username != "" {
		credential.Username = c.Username
	}
	if c.Password != "" {
		credential.Password = string(c.Password)
		// PasswordSet is required for GSSAPI (Kerberos) when a password is explicitly provided.
		// For other mechanisms, this field is ignored by the driver.
		credential.PasswordSet = true
	}
	if c.AuthMechanism != "" {
		credential.AuthMechanism = c.AuthMechanism
	}
	if c.AuthSource != "" {
		credential.AuthSource = c.AuthSource
	}
	if len(c.AuthMechanismProperties) > 0 {
		credential.AuthMechanismProperties = c.AuthMechanismProperties
	}
	return credential
}

func (c *Config) hostlist() []string {
	var hosts []string
	for _, ep := range c.Hosts {
		hosts = append(hosts, ep.Endpoint)
	}
	return hosts
}
