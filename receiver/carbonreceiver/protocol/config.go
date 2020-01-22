// Copyright 2019, OpenTelemetry Authors
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

package protocol

import (
	"fmt"

	"github.com/spf13/viper"
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

// parserMap has all supported parsers and their respective default configuration.
var parserMap = map[string]ParserConfig{
	"plaintext": &PlaintextParser{},
	"delimiter": &DelimiterParser{},
}

// LoadParserConfig is used to parser the parser configuration according to the
// specified parser type. It expects the passed viper to be pointing at the level
// of the Config reference.
func LoadParserConfig(v *viper.Viper, config *Config) error {
	defaultCfg, ok := parserMap[config.Type]
	if !ok {
		return fmt.Errorf("unknown parser type %s", config.Type)
	}

	config.Config = defaultCfg

	vParserCfg := v.Sub("config")
	if vParserCfg == nil {
		// Parser config section was not specified use the default config.
		return nil
	}

	if err := vParserCfg.UnmarshalExact(defaultCfg); err != nil {
		return err
	}

	return nil
}
