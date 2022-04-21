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

package operatortest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper/operatortest"

import (
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// ConfigUnmarshalTest is used for testing golden configs
type ConfigUnmarshalTest struct {
	Name      string
	Expect    interface{}
	ExpectErr bool
}

func configFromFileViaYaml(file string, config interface{}) error {
	bytes, err := ioutil.ReadFile(file) // #nosec - configs load based on user specified directory
	if err != nil {
		return fmt.Errorf("could not find config file: %s", err)
	}
	if err := yaml.Unmarshal(bytes, config); err != nil {
		return fmt.Errorf("failed to read config file as yaml: %s", err)
	}

	return nil
}

func configFromFileViaMapstructure(file string, config interface{}) error {
	bytes, err := ioutil.ReadFile(file) // #nosec - configs load based on user specified directory
	if err != nil {
		return fmt.Errorf("could not find config file: %s", err)
	}

	raw := map[string]interface{}{}

	if err := yaml.Unmarshal(bytes, raw); err != nil {
		return fmt.Errorf("failed to read data from yaml: %s", err)
	}

	dc := &mapstructure.DecoderConfig{Result: config, DecodeHook: helper.JSONUnmarshalerHook()}
	ms, err := mapstructure.NewDecoder(dc)
	if err != nil {
		return err
	}
	err = ms.Decode(raw)
	if err != nil {
		return err
	}
	return nil
}

// Run Unmarshalls yaml files and compares them against the expected.
func (c ConfigUnmarshalTest) Run(t *testing.T, config interface{}) {
	mapConfig := config
	yamlConfig := config
	yamlErr := configFromFileViaYaml(path.Join(".", "testdata", fmt.Sprintf("%s.yaml", c.Name)), yamlConfig)
	mapErr := configFromFileViaMapstructure(path.Join(".", "testdata", fmt.Sprintf("%s.yaml", c.Name)), mapConfig)

	if c.ExpectErr {
		require.Error(t, mapErr)
		require.Error(t, yamlErr)
	} else {
		require.NoError(t, yamlErr)
		require.Equal(t, c.Expect, yamlConfig)
		require.NoError(t, mapErr)
		require.Equal(t, c.Expect, mapConfig)
	}
}
