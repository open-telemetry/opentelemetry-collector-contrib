// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"fmt"

	"go.opentelemetry.io/collector/config"
)

// Config has the configuration guiding the sampler processor.
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// SamplingPercentage is the percentage rate at which traces or logs are going to be sampled. Defaults to zero, i.e.: no sample.
	// Values greater or equal 100 are treated as "sample all traces/logs".
	SamplingPercentage float32 `mapstructure:"sampling_percentage"`

	// HashSeed allows one to configure the hashing seed. This is important in scenarios where multiple layers of collectors
	// have different sampling rates: if they use the same seed all passing one layer may pass the other even if they have
	// different sampling rates, configuring different seeds avoids that.
	HashSeed uint32 `mapstructure:"hash_seed"`

	// TraceIDEnabled (logs only) allows to choose to use to sample by trace id or by a specific log record attribute.
	// By default, this option is true.
	TraceIDEnabled *bool `mapstructure:"trace_id_sampling"`
	// SamplingSource (logs only) allows to use a log record attribute designed by the `sampling_source` key
	// to be used to compute the sampling hash of the log record instead of trace id, if trace id is absent or trace id sampling is disabled.
	SamplingSource string `mapstructure:"sampling_source"`
	// SamplingPriority (logs only) allows to use a log record attribute designed by the `sampling_priority` key
	// to be used as the sampling priority of the log record.
	SamplingPriority string `mapstructure:"sampling_priority"`
}

var _ config.Processor = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	if cfg.SamplingPercentage < 0 {
		return fmt.Errorf("negative sampling rate: %.2f", cfg.SamplingPercentage)
	}
	return nil
}
