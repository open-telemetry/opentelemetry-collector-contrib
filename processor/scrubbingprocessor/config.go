package scrubbingprocessor

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
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

// Config defines the configuration for Resource processor.
type Config struct {
	adapter.BaseConfig `mapstructure:",squash"`
	Masking            []MaskingSettings `mapstructure:"masking"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {

	return nil
}
