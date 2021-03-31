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

package agent

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-log-collection/pipeline"
)

// Config is the configuration of the stanza log agent.
type Config struct {
	Pipeline pipeline.Config `json:"pipeline"                yaml:"pipeline"`
}

// NewConfigFromFile will create a new agent config from a YAML file.
func NewConfigFromFile(file string) (*Config, error) {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("could not find config file: %s", err)
	}

	config := Config{}
	if err := yaml.UnmarshalStrict(contents, &config); err != nil {
		return nil, fmt.Errorf("failed to read config file as yaml: %s", err)
	}

	return &config, nil
}

// NewConfigFromGlobs will create an agent config from multiple files matching a pattern.
func NewConfigFromGlobs(globs []string) (*Config, error) {
	paths := make([]string, 0, len(globs))
	for _, glob := range globs {
		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, err
		}
		paths = append(paths, matches...)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("No config files found")
	}

	config := &Config{}
	for _, path := range paths {
		newConfig, err := NewConfigFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load config from %s: %s", path, err)
		}

		config = mergeConfigs(config, newConfig)
	}

	return config, nil
}

// mergeConfigs will merge two agent configs.
func mergeConfigs(dst *Config, src *Config) *Config {
	dst.Pipeline = append(dst.Pipeline, src.Pipeline...)
	return dst
}
