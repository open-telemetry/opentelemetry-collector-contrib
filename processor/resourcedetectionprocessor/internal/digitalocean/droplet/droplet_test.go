// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package droplet

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	dometa "github.com/digitalocean/go-metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/digitalocean/droplet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

var _ droplet.Provider = (*mockMetadata)(nil)

const dropletMetadata = `{
	"droplet_id": 2756294,
	"hostname": "sample-droplet",
	"vendor_data": "#cloud-config\ndisable_root: false\nmanage_etc_hosts: true\n\ncloud_config_modules:\n - ssh\n - set_hostname\n - [ update_etc_hosts, once-per-instance ]\n\ncloud_final_modules:\n - scripts-vendor\n - scripts-per-once\n - scripts-per-boot\n - scripts-per-instance\n - scripts-user\n",
	"public_keys": [
	  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCcbi6cygCUmuNlB0KqzBpHXf7CFYb3VE4pDOf/RLJ8OFDjOM+fjF83a24QktSVIpQnHYpJJT2pQMBxD+ZmnhTbKv+OjwHSHwAfkBullAojgZKzz+oN35P4Ea4J78AvMrHw0zp5MknS+WKEDCA2c6iDRCq6/hZ13Mn64f6c372JK99X29lj/B4VQpKCQyG8PUSTFkb5DXTETGbzuiVft+vM6SF+0XZH9J6dQ7b4yD3sOder+M0Q7I7CJD4VpdVD/JFa2ycOS4A4dZhjKXzabLQXdkWHvYGgNPGA5lI73TcLUAueUYqdq3RrDRfaQ5Z0PEw0mDllCzhk5dQpkmmqNi0F sammy@digitalocean.com"
	],
	"region": "nyc3",
	"interfaces": {
	  "private": [
		{
		  "ipv4": {
			"ip_address": "104.131.20.105",
			"netmask": "255.255.192.0",
			"gateway": "104.131.0.1"
		  },
		  "mac": "04:01:2a:0f:2a:01",
		  "Type": "private"
		}
	  ],
	  "public": [
		{
		  "ipv4": {
			"ip_address": "104.131.20.105",
			"netmask": "255.255.192.0",
			"gateway": "104.131.0.1"
		  },
		  "ipv6": {
			"ip_address": "2604:A880:0800:0010:0000:0000:017D:2001",
			"cidr": 64,
			"gateway": "2604:A880:0800:0010:0000:0000:0000:0001"
		  },
		  "mac": "04:01:2a:0f:2a:01",
		  "Type": "public"
		}
	  ]
	},
	"floating_ip": {
	  "ipv4": {
		"active": false
	  }
	},
	"reserved_ip": {
	  "ipv4": {
		"active": false
	  }
	},
	"dns": {
	  "nameservers": [
		"2001:4860:4860::8844",
		"2001:4860:4860::8888",
		"8.8.8.8"
	  ]
	},
	"features": {
	  "dhcp_enabled": true
	}
  }`

type mockMetadata struct {
	mock.Mock
}

func (m *mockMetadata) Get(context.Context) (*dometa.Metadata, error) {
	meta := &dometa.Metadata{}
	err := json.NewDecoder(strings.NewReader(dropletMetadata)).Decode(meta)
	return meta, err
}

func TestDetect(t *testing.T) {
	md := &mockMetadata{}

	detector, err := NewDetector(processortest.NewNopCreateSettings(), CreateDefaultConfig())
	require.NoError(t, err)
	detector.(*Detector).metadataProvider = md
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, conventions.SchemaURL, schemaURL)

	expected := map[string]any{
		conventions.AttributeCloudProvider: "digital_ocean",
		conventions.AttributeCloudPlatform: "droplet",
		conventions.AttributeCloudRegion:   "nyc3",
		"cloud.resource_id":                "2756294",
		conventions.AttributeHostName:      "sample-droplet",
		conventions.AttributeHostID:        "2756294",
	}

	assert.Equal(t, expected, res.Attributes().AsRaw())
}
