package fiddlerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fiddlerreceiver"

import (
	"fmt"
	"time"
)

type Config struct {
	Endpoint string `mapstructure:"endpoint"`
	Token    string `mapstructure:"token"`
	Interval string `mapstructure:"interval"`
}

func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		return fmt.Errorf("endpoint must be specified")
	}

	if cfg.Token == "" {
		return fmt.Errorf("token must be specified")
	}

	if cfg.Interval == "" {
		cfg.Interval = defaultInterval
		return nil
	}

	interval, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		return fmt.Errorf("invalid interval: %v", err)
	}

	if interval.Minutes() < 5 {
		return fmt.Errorf("interval must be at least 5 minutes")
	}

	return nil
}
