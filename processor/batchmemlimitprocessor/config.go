package batchmemlimitprocessor

import (
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

// Config defines the configuration for Resource processor.
type Config struct {
	adapter.BaseConfig `mapstructure:",squash"`

	// Timeout sets the time after which a batch will be sent regardless of size.
	Timeout time.Duration `mapstructure:"timeout"`

	// SendMemorySize is the size of batch in bytes which, after hit, will trigger it to be sent
	SendMemorySize uint64 `mapstructure:"batch_mem_limit"`

	// SendBatchSize is the size of a batch which, after hit, will trigger it to be sent.
	SendBatchSize uint64 `mapstructure:"send_batch_size"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	return nil
}
