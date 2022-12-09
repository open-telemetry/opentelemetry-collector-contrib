package scrubbingprocessor

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
)

type AttributeType string

const (
	EmptyAttribute    AttributeType = ""
	ResourceAttribute               = "resource"
	RecordAttribute                 = "record"
)

type MaskingSettings struct {
	AttributeType AttributeType `mapstructure:"attribute_type"`
	AttributeKey  string        `mapstructure:"attribute_key"`
	Regexp        string        `mapstructure:"regexp"`
	Placeholder   string        `mapstructure:"placeholder"`
}

// Config defines configuration for Resource processor.
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	adapter.BaseConfig       `mapstructure:",squash"`
	Masking                  []MaskingSettings `mapstructure:"masking"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {

	return nil
}
