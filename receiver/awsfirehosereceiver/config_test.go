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

package awsfirehosereceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join(".", "testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.Equal(t, 1, len(cfg.Receivers))

	r := cfg.Receivers[config.NewComponentID(typeStr)].(*Config)
	require.Equal(t, &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		RecordType:       "cwmetrics",
		AccessKey:        "some_access_key",
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "0.0.0.0:4433",
			TLSSetting: &configtls.TLSServerSetting{
				TLSSetting: configtls.TLSSetting{
					CertFile: "server.crt",
					KeyFile:  "server.key",
				},
			},
		},
	}, r)
}
