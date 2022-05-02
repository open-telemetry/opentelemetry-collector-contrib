package lmexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lmexporter"

import (
	"fmt"
	"net/url"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

//Config defines configuration for Logic Monitor exporter.
type Config struct {
	config.ExporterSettings `mapstructure:",squash"`

	confighttp.HTTPClientSettings `mapstructure:",squash"`

	exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`
	ResourceToTelemetrySettings  resourcetotelemetry.Settings `mapstructure:"resource_to_telemetry_conversion"`

	// ApiToken of Logicmonitor
	APIToken map[string]string `mapstructure:"apitoken"`

	// URL on which the data should be forwarded
	URL string `mapstructure:"url"`

	// Headers for POST requests
	Headers map[string]string `mapstructure:"headers"`
}

// Validate checks whether the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.URL != "" {
		u, err := url.Parse(cfg.URL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("URL must be valid")
		}
	}
	return nil
}

var _ config.Exporter = (*Config)(nil)
