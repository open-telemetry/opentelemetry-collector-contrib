// Copyright The OpenTelemetry Authors
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

package protocol // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"

import (
	"fmt"
	"sort"

	"go.opentelemetry.io/collector/confmap"
)

var (
	// parserMap has all supported parsers and their respective default
	// configuration.
	parserMap = map[string]func() ParserConfig{
		"plaintext": plaintextDefaultConfig,
		"regex":     regexDefaultConfig,
	}

	// validParsers keeps a list of all valid parsers to be used in error
	// messages. It is initialized on init so new valid parsers just need to be
	// added at parserMap above.
	validParsers []string
)

func init() {
	for k := range parserMap {
		validParsers = append(validParsers, k)
	}

	// Sort the valid parsers by name so the message is consistent on every run.
	sort.Slice(validParsers, func(i, j int) bool {
		return validParsers[i] < validParsers[j]
	})
}

const (
	// configSection is the name that must be used for the config settings
	// in the configuration struct. The metadata mapstructure for the parser
	// should use the same string.
	configSection = "config"
)

// Config is the general configuration for the parser to be used.
type Config struct {
	// Type of the parser to be used with the arriving data.
	Type string `mapstructure:"type"`

	// Config placeholder for the configuration object of the selected parser.
	Config ParserConfig `mapstructure:"config"`
}

// ParserConfig is the configuration of a given parser.
type ParserConfig interface {
	// BuildParser builds the respective parser of the configuration instance.
	BuildParser() (Parser, error)
}

// LoadParserConfig is used to load the parser configuration according to the
// specified parser type. It expects the passed viper to be pointing at the level
// of the Config reference.
func LoadParserConfig(cp *confmap.Conf, cfg *Config) error {
	defaultCfgFn, ok := parserMap[cfg.Type]
	if !ok {
		return fmt.Errorf(
			"unknown parser type %q, valid parser types: %v",
			cfg.Type,
			validParsers)
	}

	cfg.Config = defaultCfgFn()

	vParserCfg, errSub := cp.Sub(configSection)
	if errSub != nil {
		return errSub
	}

	return vParserCfg.Unmarshal(cfg.Config, confmap.WithErrorUnused())
}
