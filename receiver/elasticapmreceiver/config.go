package elasticapmreceiver

import (
	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	*confighttp.ServerConfig `mapstructure:",squash"`
	EventsURLPath            string `mapstructure:"events_url_path"`
	RUMEventsUrlPath         string `mapstructure:"rum_events_url_path"`
	MaxEventSize             int    `mapstructure:"max_event_size_bytes"`
	BatchSize                int    `mapstructure:"batch_size"`
}

func (cfg *Config) Validate() error {
	return nil
}
