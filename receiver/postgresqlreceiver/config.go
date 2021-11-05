package postgresqlreceiver

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"
)

type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	Username                                string   `mapstructure:"username"`
	Password                                string   `mapstructure:"password"`
	Databases                               []string `mapstructure:"databases"`
	Host                                    string   `mapstructure:"host"`
	Port                                    int      `mapstructure:"port"`
	SSLConfig                               `mapstructure:",squash"`
}

type SSLConfig struct {
	SSLMode     string `mapstructure:"ssl_mode"`
	SSLRootCert string `mapstructure:"ssl_root_cert"`
	SSLCert     string `mapstructure:"ssl_cert"`
	SSLKey      string `mapstructure:"ssl_key"`
}

func (c *SSLConfig) Validate() []error {
	var errs []error
	validValues := map[string]struct{}{
		"require":     {},
		"verify-ca":   {},
		"verify-full": {},
		"disable":     {},
	}

	if _, ok := validValues[c.SSLMode]; !ok {
		errs = append(errs,
			fmt.Errorf("SSL Mode '%s' not supported, valid values are 'require', 'verify-ca', 'verify-full', 'disable'. The default is 'require'", c.SSLMode))
	}

	return errs
}

// Leveraged to configure SSL in the connection string
func (c *SSLConfig) ConnString() string {
	conn := fmt.Sprintf("sslmode='%s'", c.SSLMode)
	if c.SSLMode == "disable" {
		return conn
	}

	if c.SSLRootCert != "" {
		conn += fmt.Sprintf(" sslrootcert='%s'", c.SSLRootCert)
	}

	if c.SSLCert != "" {
		conn += fmt.Sprintf(" sslcert='%s'", c.SSLCert)
	}

	if c.SSLKey != "" {
		conn += fmt.Sprintf(" sslkey='%s'", c.SSLKey)
	}

	return conn
}

// Errors for missing required config parameters.
const (
	ErrNoUsername = "invalid config: missing username"
	ErrNoPassword = "invalid config: missing password"
)

func (cfg *Config) Validate() error {
	var errs []error
	if cfg.Username == "" {
		errs = append(errs, errors.New(ErrNoUsername))
	}
	if cfg.Password == "" {
		errs = append(errs, errors.New(ErrNoPassword))
	}

	errs = append(errs, cfg.SSLConfig.Validate()...)
	return multierr.Combine(errs...)
}
