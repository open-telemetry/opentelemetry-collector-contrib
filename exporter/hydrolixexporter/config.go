package hydrolixexporter

import (
	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	confighttp.ClientConfig `mapstructure:",squash"`

	// Hydrolix-specific configuration
	HDXTable     string `mapstructure:"hdx_table"`
	HDXTransform string `mapstructure:"hdx_transform"`
	HDXUsername  string `mapstructure:"hdx_username"`
	HDXPassword  string `mapstructure:"hdx_password"`
}
