// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package intracesampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intracesamplerprocessor"

import (
	"fmt"
)

type Config struct {

	// SamplingPercentage is the percentage rate at which spans in a trace are going to be sampled.
	// Defaults to zero, i.e.: no sample (remove spans from the trace by configuration).
	// Values greater or equal 100 are treated as "sample all spans".
	SamplingPercentage float64 `mapstructure:"sampling_percentage"`

	// HashSeed allows one to configure the hashing seed. This is important in scenarios where multiple layers of collectors
	// have different sampling rates: if they use the same seed all passing one layer may pass the other even if they have
	// different sampling rates, configuring different seeds avoids that.
	HashSeed uint32 `mapstructure:"hash_seed"`

	// ScopeLeaves will unsample spans from this scope, if they have no sampled descendants
	ScopeLeaves []string `mapstructure:"scope_leaves"`
}

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	if cfg.SamplingPercentage < 0 {
		return fmt.Errorf("negative sampling rate: %.2f", cfg.SamplingPercentage)
	}
	return nil
}
