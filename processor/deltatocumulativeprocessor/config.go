package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

var _ component.ConfigValidator = (*Config)(nil)

type Config struct {
	Interval time.Duration
}

func (c *Config) Validate() error {
	if c.Interval <= time.Duration(0) {
		return fmt.Errorf("delta aggregation interval must be >0s")
	}
	return nil
}
