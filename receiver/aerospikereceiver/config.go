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
	defaultEndpoint              = "localhost:3000"
	defaultTimeout               = 20 * time.Second
	defaultCollectClusterMetrics = false
)

var (
	errBadEndpoint   = errors.New("endpoint must be specified as host:port")
	errBadPort       = errors.New("invalid port in endpoint")
	errEmptyEndpoint = errors.New("endpoint must be specified")
	errEmptyPassword = errors.New("password must be set if username is set")
	errEmptyUsername = errors.New("username must be set if password is set")
)

// Config is the receiver configuration
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	Endpoint                                string                     `mapstructure:"endpoint"`
	Username                                string                     `mapstructure:"username"`
	Password                                string                     `mapstructure:"password"`
	CollectClusterMetrics                   bool                       `mapstructure:"collect_cluster_metrics"`
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

	host, portStr, err := net.SplitHostPort(c.Endpoint)
	if err != nil {
		return multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadEndpoint, err))
	}

	if host == "" {
		allErrs = multierr.Append(allErrs, errBadEndpoint)
	}

	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %s", errBadPort, err))
	}

	if port < 0 || port > 65535 {
		allErrs = multierr.Append(allErrs, fmt.Errorf("%w: %d", errBadPort, port))
	}

	if c.Username != "" && c.Password == "" {
		allErrs = multierr.Append(allErrs, errEmptyPassword)
	}

	if c.Password != "" && c.Username == "" {
		allErrs = multierr.Append(allErrs, errEmptyUsername)
	}

	return allErrs
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentID(typeStr)),
			CollectionInterval: time.Minute,
		},
		Endpoint:              defaultEndpoint,
		Timeout:               defaultTimeout,
		CollectClusterMetrics: defaultCollectClusterMetrics,
		TLS:                   configtls.TLSClientSetting{},
		Metrics:               metadata.DefaultMetricsSettings(),
	}
}
