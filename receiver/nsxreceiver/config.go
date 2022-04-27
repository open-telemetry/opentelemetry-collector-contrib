package nsxreceiver

import (
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxreceiver/internal/metadata"
)

// Config is the configuraiton for the NSX receiver
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`
	MetricsConfig           MetricsConfig `mapstructure:"metrics"`
}

// MetricsConfig is the metrics configuration portion of the nsxreceiver
type MetricsConfig struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	confighttp.HTTPClientSettings           `mapstructure:",squash"`
	Settings                                metadata.MetricsSettings `mapstructure:"settings"`
	Username                                string                   `mapstructure:"username"`
	Password                                string                   `mapstructure:"password"`
}

// Validate returns if the NSX configuration is valid
func (c *Config) Validate() error {
	return nil
}

// ID returns the underlying MetricsConfig's ID
func (c *Config) ID() config.ComponentID {
	return c.MetricsConfig.ID()
}
