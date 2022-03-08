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

package schemaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor"

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Masterminds/semver/v3"
	"go.opentelemetry.io/collector/config"
)

// Config specifies the set of attributes to be inserted, updated, upserted and
// deleted and the properties to include/exclude a span from being processed.
// This processor handles all forms of modifications to attributes within a span.
// Prior to any actions being applied, each span is compared against
// the include properties and then the exclude properties if they are specified.
// This determines if a span is to be processed or not.
// The list of actions is applied in order specified in the configuration.
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
	Transform                []TransformConfig        `mapstructure:"transform"`
}

type TransformConfig struct {
	From string `mapstructure:"from"`
	To   string `mapstructure:"to"`
}

var _ config.Processor = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {

	if len(cfg.Transform) == 0 {
		return fmt.Errorf("'transform' must contain at least one element")
	}

	for _, transform := range cfg.Transform {
		if len(transform.From) == 0 {
			return fmt.Errorf("'from' Schema URL must be specified")
		}
		if len(transform.To) == 0 {
			return fmt.Errorf("'to' Schema URL must be specified")
		}

		if transform.From == transform.To {
			return fmt.Errorf("'from' and 'to' Schema URLs must be different (%s)", transform.From)
		}

		fromFamily, _, err := splitSchemaURL(transform.From)
		if err != nil {
			return err
		}

		toFamily, toVersion, err := splitSchemaURL(transform.To)
		if err != nil {
			return err
		}

		if fromFamily != toFamily {
			return fmt.Errorf("'from' and 'to' Schema Families do not match (%s, %s)", transform.From, transform.To)
		}

		_, err = semver.StrictNewVersion(toVersion)
		if err != nil {
			return fmt.Errorf("'to' Schema URL is invalid, must end with version number (%s)", transform.To)
		}
	}

	return nil
}

func splitSchemaURL(schemaURL string) (family string, version string, err error) {
	_, err = url.Parse(schemaURL)
	if err != nil {
		return "", "", err
	}

	i := strings.LastIndex(schemaURL, "/")
	if i < 0 {
		return "", "", errors.New("invalid schema URL, must have at least one forward slash")
	}

	family = schemaURL[0:i]
	version = schemaURL[i+1:]
	return
}
