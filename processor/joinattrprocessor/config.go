// Copyright  OpenTelemetry Authors
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

package joinattrprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/joinattrprocessor"

import (
	"go.opentelemetry.io/collector/config"
)

type Config struct {
	config.ProcessorSettings `mapstructure:",squash"`

	// JoinAttributes are the attributes to be joined
	JoinAttributes []string `mapstructure:"attributes"`

	// Attribute containing the final value
	TargetAttribute string `mapstructure:"target_attribute"`

	// Attributes will be the joined using the separator
	Separator string `mapstructure:"separator"`

	// Whether to override an existing attribute or not. Default is false.
	Override bool `mapstructure:"override"`
}
