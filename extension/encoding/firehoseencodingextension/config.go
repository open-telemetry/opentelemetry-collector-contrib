package firehoseencodingextension

import (
	"fmt"
	"go.opentelemetry.io/collector/component"
)

type contentEncoding string

const (
	NoEncoding  = ""
	GZipEncoded = "gzip"
)

type Config struct {
	Logs    `mapstructure:"logs"`
	Metrics `mapstructure:"metrics"`
}

type Logs struct {
	LogsEncoding contentEncoding `mapstructure:"content_encoding"`
}

type Metrics struct {
	MetricsEncoding contentEncoding `mapstructure:"content_encoding"`
}

func createDefaultConfig() component.Config {
	return &Config{
		Metrics: Metrics{
			MetricsEncoding: NoEncoding,
		},
		Logs: Logs{
			LogsEncoding: GZipEncoded,
		},
	}
}

func (c *Config) Validate() error {
	switch c.MetricsEncoding {
	case NoEncoding:
	case GZipEncoded:
	default:
		return fmt.Errorf("unknown metrics encoding %s", c.MetricsEncoding)
	}
	switch c.LogsEncoding {
	case NoEncoding:
	case GZipEncoded:
	default:
		return fmt.Errorf("unknown logs encoding %s", c.LogsEncoding)
	}
	return nil
}
