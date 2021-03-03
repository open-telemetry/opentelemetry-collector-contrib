// Copyright 2021, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package telegrafreceiver

import (
	"go.opentelemetry.io/collector/config/configmodels"
)

// Config defines configuration for the telegraf receiver.
type Config struct {
	configmodels.ReceiverSettings `mapstructure:",squash"`

	// AgentConfig is the yaml config used as telegraf configuration.
	// Please note that only inputs should be configured as all metrics gathered
	// by them will be passed through to otc pipeline for processing and export.
	AgentConfig string `mapstructure:"agent_config"`

	// SeparateField controls whether the ingested metrics should have a field
	// concatenated with metric name like e.g. metric=mem_available or maybe rather
	// have it as a separate label like e.g. metric=mem field=available
	SeparateField bool `mapstructure:"separate_field"`
}
