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

	// Hosts are a list of <host>:<port>
	// for standalone deployments, use the hostname and port of the mongod instance
	// for sharded deployments it is the list of mongos instances
	// for replica set deployments
	Hosts []confignet.TCPAddr `mapstructure:"hosts"`
	// Username to use for authentication; optional
	Username string `mapstructure:"username"`
	// Password to use for authentication; required if Username is present
	Password string `mapstructure:"password"`
	// ReplicaSet is an optional parameter that if supplied will allow autodiscovery of
	// any replica set members when given the address of any one valid member
	ReplicaSet string `mapstructure:"replica_set,omitempty"`

	Timeout time.Duration `mapstructure:"timeout"`
}

func (c *Config) Validate() error {
	if len(c.Hosts) == 0 {
		return errors.New("no hosts were specified in the config")
	}

	var err error
	for _, host := range c.Hosts {
		_, _, hpErr := net.SplitHostPort(host.Endpoint)
		if hpErr != nil {
			err = multierr.Append(err, fmt.Errorf("endpoint '%s' does not match format of '<host>:<port>': %w", host.Endpoint, hpErr))
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
