// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "udp_endpoint"),
			expected: &Config{
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
			id: component.NewIDWithName(metadata.Type, "proxy_server"),
			expected: &Config{
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
