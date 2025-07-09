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
	"go.opentelemetry.io/collector/confmap/xconfmap"

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
				AddrConfig: confignet.AddrConfig{
					Endpoint:  "0.0.0.0:5678",
					Transport: confignet.TransportTypeUDP,
				},
				ProxyServer: &proxy.Config{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "localhost:2000",
					},
					ProxyAddress: "",
					TLS: configtls.ClientConfig{
						Insecure:   false,
						ServerName: "",
					},
					Region:      "",
					RoleARN:     "",
					AWSEndpoint: "",
					ServiceName: "xray",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "proxy_server"),
			expected: &Config{
				AddrConfig: confignet.AddrConfig{
					Endpoint:  "localhost:2000",
					Transport: confignet.TransportTypeUDP,
				},
				ProxyServer: &proxy.Config{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "0.0.0.0:1234",
					},
					ProxyAddress: "https://proxy.proxy.com",
					TLS: configtls.ClientConfig{
						Insecure:   true,
						ServerName: "something",
					},
					Region:      "us-west-1",
					RoleARN:     "arn:aws:iam::123456789012:role/awesome_role",
					AWSEndpoint: "https://another.aws.endpoint.com",
					LocalMode:   true,
					ServiceName: "xray",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
