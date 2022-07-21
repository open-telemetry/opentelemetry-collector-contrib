package scrubbingprocessor

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"go.opentelemetry.io/collector/config"
)

type MaskingSettings struct {
	Regexp      string `mapstructure:"regexp"`
	Placeholder string `mapstructure:"placeholder"`
}

// Config defines configuration for Resource processor.
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	adapter.BaseConfig       `mapstructure:",squash"`
	Masking                  []MaskingSettings `mapstructure:"masking"`
}

var _ config.Processor = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {

	return nil
}
