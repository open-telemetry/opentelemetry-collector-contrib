package datadogrumreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogrumreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`
	// ReadTimeout of the http server
	ReadTimeout time.Duration `mapstructure:"read_timeout"`
}
