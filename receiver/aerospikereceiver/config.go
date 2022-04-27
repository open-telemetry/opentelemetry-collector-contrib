package aerospikereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/internal/metadata"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"
)

var (
	defaultEndpoint      = "localhost:3000"
	defaultTimeout       = 20 * time.Second
	defaultDiscoverNodes = false
)

var (
	errEmptyEndpoint = errors.New("endpoint must be specified")
	errBadEndpoint   = errors.New("endpoint must be specified as host:port")
)

// Config is the receiver configuration
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	Endpoint                                string                     `mapstructure:"endpoint"`
	Username                                string                     `mapstructure:"username"`
	Password                                string                     `mapstructure:"password"`
	DiscoverNodes                           bool                       `mapstructure:"discover_nodes"`
	Timeout                                 time.Duration              `mapstructure:"timeout"`
	TLS                                     configtls.TLSClientSetting `mapstructure:"tls,omitempty"`
	Metrics                                 metadata.MetricsSettings   `mapstructure:"metrics"`
}

// Validate validates the values of the given Config, and returns an error if validation fails
func (c *Config) Validate() error {
	var allErrs error

	if c.Endpoint == "" {
		return multierr.Append(allErrs, errEmptyEndpoint)
	}

	_, portStr, err := net.SplitHostPort(c.Endpoint)
	if err != nil {
		return multierr.Append(allErrs, fmt.Errorf("invalid endpoint '%s': %w", c.Endpoint, err))
	}

	if _, err := strconv.ParseInt(portStr, 10, 32); err != nil {
		return multierr.Append(allErrs, fmt.Errorf("invalid port '%s': %w", portStr, err))
	}

	return allErrs
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
			CollectionInterval: time.Minute,
		},
		Endpoint:      defaultEndpoint,
		Timeout:       defaultTimeout,
		DiscoverNodes: defaultDiscoverNodes,
		TLS:           configtls.TLSClientSetting{},
		Metrics:       metadata.DefaultMetricsSettings(),
	}
}
