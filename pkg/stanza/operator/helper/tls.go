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

package helper

import (
	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/collector/config/configtls"
)

type TLSServerConfig struct {
	*configtls.TLSServerSetting `mapstructure:",squash" json:",inline" yaml:",inline"`
}

func NewTLSServerConfig(setting *configtls.TLSServerSetting) *TLSServerConfig {
	return &TLSServerConfig{
		TLSServerSetting: setting,
	}
}

func (t *TLSServerConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var tlsConfig map[string]interface{}
	err := unmarshal(&tlsConfig)
	if err != nil {
		return err
	}
	return mapstructure.Decode(tlsConfig, &t.TLSServerSetting)
}
