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
	confignet.TCPAddr                       `mapstructure:",squash"`
	configtls.TLSClientSetting              `mapstructure:"tls,omitempty"`
	Username                                string        `mapstructure:"username"`
	Password                                string        `mapstructure:"password"`
	Timeout                                 time.Duration `mapstructure:"timeout"`
}

func (c *Config) Validate() error {
	var err error
	if c.Endpoint == "" {
		err = multierr.Append(err, errors.New("no endpoint specified"))
	} else {
		_, _, hpErr := net.SplitHostPort(c.Endpoint)
		if hpErr != nil {
			err = multierr.Append(err, fmt.Errorf("endpoint does not match format of 'host:port': %w", hpErr))
		}
	}

	if c.Username != "" && c.Password == "" {
		err = multierr.Append(err, errors.New("user provided without password"))
	} else if c.Username == "" && c.Password != "" {
		err = multierr.Append(err, errors.New("password provided without user"))
	}

	if _, tlsErr := c.LoadTLSConfig(); tlsErr != nil {
		err = multierr.Append(err, fmt.Errorf("error loading tls config: %w", tlsErr))
	}

	return err
}
