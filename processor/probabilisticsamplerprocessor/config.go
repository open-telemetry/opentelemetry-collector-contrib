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
	"go.opentelemetry.io/collector/model/pdata"
)

var severityTextToNum = map[string]pdata.SeverityNumber{
	"default": pdata.SeverityNumberUNDEFINED,
	"trace":   pdata.SeverityNumberTRACE,
	"trace2":  pdata.SeverityNumberTRACE2,
	"trace3":  pdata.SeverityNumberTRACE3,
	"trace4":  pdata.SeverityNumberTRACE4,
	"debug":   pdata.SeverityNumberDEBUG,
	"debug2":  pdata.SeverityNumberDEBUG2,
	"debug3":  pdata.SeverityNumberDEBUG3,
	"debug4":  pdata.SeverityNumberDEBUG4,
	"info":    pdata.SeverityNumberINFO,
	"info2":   pdata.SeverityNumberINFO2,
	"info3":   pdata.SeverityNumberINFO3,
	"info4":   pdata.SeverityNumberINFO4,
	"warn":    pdata.SeverityNumberWARN,
	"warn2":   pdata.SeverityNumberWARN2,
	"warn3":   pdata.SeverityNumberWARN3,
	"warn4":   pdata.SeverityNumberWARN4,
	"error":   pdata.SeverityNumberERROR,
	"error2":  pdata.SeverityNumberERROR2,
	"error3":  pdata.SeverityNumberERROR3,
	"error4":  pdata.SeverityNumberERROR4,
	"fatal":   pdata.SeverityNumberFATAL,
	"fatal2":  pdata.SeverityNumberFATAL2,
	"fatal3":  pdata.SeverityNumberFATAL3,
	"fatal4":  pdata.SeverityNumberFATAL4,
}

type severityPair struct {
	Level              string  `mapstructure:"severity_level"`
	SamplingPercentage float32 `mapstructure:"sampling_percentage"`
}

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

	// Severity is an array of severity and sampling percentage pairs allocating a specific sampling percentage
	// to a given severity level.
	Severity []severityPair `mapstructure:"severity"`
}

var _ config.Processor = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	keys := map[string]bool{}
	for _, pair := range cfg.Severity {
		if _, ok := severityTextToNum[pair.Level]; !ok {
			return fmt.Errorf("unrecognized severity level: %s", pair.Level)
		}
		if keys[pair.Level] {
			return fmt.Errorf("severity already used: %s", pair.Level)
		}
		keys[pair.Level] = true
	}
	return nil
}
