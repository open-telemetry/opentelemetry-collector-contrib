package solarwindsapmsettingsextension

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Endpoint string `mapstructure:"endpoint"`
	Key      string `mapstructure:"key"`
	Interval string `mapstructure:"interval"`
}

func (cfg *Config) Validate() error {
	if len(cfg.Endpoint) == 0 {
		return fmt.Errorf("endpoint must not be empty")
	}
	endpointArr := strings.Split(cfg.Endpoint, ":")
	if len(endpointArr) != 2 {
		return fmt.Errorf("endpoint should be in \"<host>:<port>\" format")
	}
	if _, err := strconv.Atoi(endpointArr[1]); err != nil {
		return fmt.Errorf("the <port> portion of endpoint has to be an integer")
	}
	if len(cfg.Key) == 0 {
		return fmt.Errorf("key must not be empty")
	}
	keyArr := strings.Split(cfg.Key, ":")
	if len(keyArr) != 2 {
		return fmt.Errorf("key should be in \"<token>:<service_name>\" format")
	}

	if _, err := time.ParseDuration(cfg.Interval); err != nil {
		return fmt.Errorf("interval has to be a duration string. Valid time units are \"ns\", \"us\" (or \"µs\"), \"ms\", \"s\", \"m\", \"h\"")
	}
	return nil
}
