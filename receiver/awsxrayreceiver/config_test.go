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

package awsxrayreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected config.Receiver
	}{
		{
			id:       config.NewComponentIDWithName(awsxray.TypeStr, ""),
			expected: createDefaultConfig(),
		},
		{
			id: config.NewComponentIDWithName(awsxray.TypeStr, "udp_endpoint"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(awsxray.TypeStr)),
				NetAddr: confignet.NetAddr{
					Endpoint:  "0.0.0.0:5678",
					Transport: "udp",
				},
				ProxyServer: &proxy.Config{
					TCPAddr: confignet.TCPAddr{
						Endpoint: "0.0.0.0:2000",
					},
					ProxyAddress: "",
					TLSSetting: configtls.TLSClientSetting{
						Insecure:   false,
						ServerName: "",
					},
					Region:      "",
					RoleARN:     "",
					AWSEndpoint: "",
				},
			},
		},
		{
			id: config.NewComponentIDWithName(awsxray.TypeStr, "proxy_server"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(awsxray.TypeStr)),
				NetAddr: confignet.NetAddr{
					Endpoint:  "0.0.0.0:2000",
					Transport: "udp",
				},
				ProxyServer: &proxy.Config{
					TCPAddr: confignet.TCPAddr{
						Endpoint: "0.0.0.0:1234",
					},
					ProxyAddress: "https://proxy.proxy.com",
					TLSSetting: configtls.TLSClientSetting{
						Insecure:   true,
						ServerName: "something",
					},
					Region:      "us-west-1",
					RoleARN:     "arn:aws:iam::123456789012:role/awesome_role",
					AWSEndpoint: "https://another.aws.endpoint.com",
					LocalMode:   true,
				},
			}},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, config.UnmarshalReceiver(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
