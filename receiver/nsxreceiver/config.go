package nsxreceiver

import (
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

// Config is the configuraiton for the NSX receiver
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`
	configtls.TLSClientSetting              `mapstructure:",squash"`
	Host                                    string `mapstructure:"host"`
	Username                                string `mapstructure:"username"`
	Password                                string `mapstructure:"password"`
}

// Validate returns if the NSX configuration is valid
func (c *Config) Validate() error {
	return nil
}
