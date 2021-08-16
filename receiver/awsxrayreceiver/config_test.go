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
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[awsxray.TypeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 3)

	// ensure default configurations are generated when users provide
	// nothing.
	r0 := cfg.Receivers[config.NewID(awsxray.TypeStr)]
	assert.Equal(t, factory.CreateDefaultConfig(), r0)

	// ensure the UDP endpoint can be properly overwritten
	r1 := cfg.Receivers[config.NewIDWithName(awsxray.TypeStr, "udp_endpoint")].(*Config)
	assert.Equal(t,
		&Config{
			ReceiverSettings: config.NewReceiverSettings(config.NewIDWithName(awsxray.TypeStr, "udp_endpoint")),
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
		r1)

	// ensure the fields under proxy_server are properly overwritten
	r2 := cfg.Receivers[config.NewIDWithName(awsxray.TypeStr, "proxy_server")].(*Config)
	assert.Equal(t,
		&Config{
			ReceiverSettings: config.NewReceiverSettings(config.NewIDWithName(awsxray.TypeStr, "proxy_server")),
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
		},
		r2)
}
