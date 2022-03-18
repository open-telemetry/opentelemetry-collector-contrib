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

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset/regexp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

// Config relating to Process Metric Scraper.
type Config struct {
	internal.ConfigSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// Metrics allows to customize scraped metrics representation.
	Metrics metadata.MetricsSettings `mapstructure:"metrics"`
	// Include specifies a filter on the process names that should be included from the generated metrics.
	// Exclude specifies a filter on the process names that should be excluded from the generated metrics.
	// If neither `include` or `exclude` are set, process metrics will be generated for all processes.
	Include MatchConfig `mapstructure:"include"`
	Exclude MatchConfig `mapstructure:"exclude"`

	// MuteProcessNameError is a flag that will mute the error encountered when trying to read a process the
	// collector does not have permission for.
	// See https://github.com/open-telemetry/opentelemetry-collector/issues/3004 for more information.
	MuteProcessNameError bool `mapstructure:"mute_process_name_error,omitempty"`

	Filters []FilterConfig `mapstructure:"filters"`
}

type MatchConfig struct {
	filterset.Config `mapstructure:",squash"`

	Names []string `mapstructure:"names"`
}

type FilterConfig struct {
	ExecutableName string `mapstructure:"executable_name"`
	ExecutablePath string `mapstructure:"executable_path"`
	Command string `mapstructure:"process_command"`
	CommandLine string `mapstructure:"process_command_line"`
	Owner string `mapstructure:"process_owner"`
	PID int32 `mapstructure:"process_pid"`
	RegexpConfig *regexp.Config `mapstructure:"regexp"`
}
