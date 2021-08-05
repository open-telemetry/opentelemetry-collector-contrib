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

package deltatorateprocessor

import (
	"fmt"
	"sort"

	"go.opentelemetry.io/collector/config"
)

// Config defines the configuration for the processor.
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// List of delta sum metrics to convert to rates
	Metrics []string `mapstructure:"metrics"`

	// Time unit to calculate the rate over. This could be one of [ns, ms, s, m]. Default is s.
	TimeUnit StringTimeUnit `mapstructure:"time_unit"`
}

type StringTimeUnit string

const (
	nanosecond  StringTimeUnit = "ns"
	millisecond StringTimeUnit = "ms"
	second      StringTimeUnit = "s"
	minute      StringTimeUnit = "m"
)

var timeUnits = map[StringTimeUnit]struct{}{
	nanosecond:  {},
	millisecond: {},
	second:      {},
	minute:      {},
}

var validTimeUnits = func() []string {
	ret := make([]string, len(timeUnits))
	i := 0
	for k := range timeUnits {
		ret[i] = string(k)
		i++
	}
	sort.Strings(ret)
	return ret
}

func (stu StringTimeUnit) isValid() bool {
	_, ok := timeUnits[stu]
	return ok
}

// Validate checks whether the input configuration has all of the required fields for the processor.
// An error is returned if there are any invalid inputs.
func (config *Config) Validate() error {
	if len(config.Metrics) == 0 {
		return fmt.Errorf("metric names are missing")
	}

	if config.TimeUnit != "" && !config.TimeUnit.isValid() {
		return fmt.Errorf("time_unit must be in %q", validTimeUnits())
	}
	return nil
}
