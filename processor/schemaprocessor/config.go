// Copyright  The OpenTelemetry Authors
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

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/config"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/schema"
)

var (
	errRequiresTargets  = errors.New("requires schema targets")
	errDuplicateTargets = errors.New("duplicate targets detected")
)

// Config defines the user provided values for the Schema Processor
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"`

	// PreCache is a list of schema URLs that are downloaded
	// and cached at the start of the collector runtime
	// in order to avoid fetching data that later on could
	// block processing of signals. (Optional field)
	Prefetch []string `mapstructure:"prefetch"`

	// Targets define what schema families should be
	// translated to, allowing older and newer formats
	// to conform to the target schema identifier.
	Targets []string `mapstructure:"targets"`
}

func (c *Config) Validate() error {
	for _, schemaURL := range c.Prefetch {
		_, _, err := schema.GetFamilyAndIdentifier(schemaURL)
		if err != nil {
			return err
		}
	}
	// Not strictly needed since it would just pass on
	// any data that doesn't match targets, however defining
	// this processor with no targets is wasteful.
	if len(c.Targets) == 0 {
		return fmt.Errorf("no schema targets defined: %w", errRequiresTargets)
	}

	families := make(map[string]struct{})
	for _, target := range c.Targets {
		family, _, err := schema.GetFamilyAndIdentifier(target)
		if err != nil {
			return err
		}
		if _, exist := families[family]; exist {
			return errDuplicateTargets
		}
		families[family] = struct{}{}
	}

	return nil
}
