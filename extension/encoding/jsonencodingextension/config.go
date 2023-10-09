package jsonencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonencodingextension"

type Config struct{}

func (c *Config) Validate() error {
	return nil
}
