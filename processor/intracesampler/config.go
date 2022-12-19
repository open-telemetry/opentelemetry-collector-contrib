package intracesampler

import (
	"fmt"

	"go.opentelemetry.io/collector/config"
)

type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// SamplingPercentage is the percentage rate at which spans in a trace are going to be sampled.
	// Defaults to zero, i.e.: no sample (remove spans from the trace by configuration).
	// Values greater or equal 100 are treated as "sample all spans".
	SamplingPercentage float64 `mapstructure:"sampling_percentage"`

	// HashSeed allows one to configure the hashing seed. This is important in scenarios where multiple layers of collectors
	// have different sampling rates: if they use the same seed all passing one layer may pass the other even if they have
	// different sampling rates, configuring different seeds avoids that.
	HashSeed uint32 `mapstructure:"hash_seed"`

	// will unsample spans from this scope, if they have no sampled decendants
	ScopeLeaves []string `mapstructure:"scope_leaves"`
}

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	if cfg.SamplingPercentage < 0 {
		return fmt.Errorf("negative sampling rate: %.2f", cfg.SamplingPercentage)
	}
	return nil
}
