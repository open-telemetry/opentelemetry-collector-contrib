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

package jqprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/jqprocessor"

import (
	"go.opentelemetry.io/collector/component"
)

// Config is the configuration for the processor.
type Config struct {

	// JqStatement describes the jq statement to be run on target json.
	JqStatement string `mapstructure:"jq_statement"`
}

var _ component.Config = (*Config)(nil)
