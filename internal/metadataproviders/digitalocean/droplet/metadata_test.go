// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package droplet

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	dometa "github.com/digitalocean/go-metadata"
	"github.com/stretchr/testify/assert"
)

func TestNewProvider(t *testing.T) {
	provider := NewProvider(dometa.NewClient())
	assert.NotNil(t, provider)
}

func TestQueryEndpointFailed(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()

	withServer(t, http.NotFoundHandler(), func(provider *dropletProviderImpl) {
		_, err := provider.Get(context.Background())
		assert.Error(t, err)
	})
}

func TestQueryEndpointMalformed(t *testing.T) {
	malformedHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprintln(w, "{")
		assert.NoError(t, err)
	})

	withServer(t, malformedHandler, func(provider *dropletProviderImpl) {
		_, err := provider.Get(context.Background())
		assert.Error(t, err)
	})
}

func TestQueryEndpointCorrect(t *testing.T) {
	metadata := `{
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

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(metadata))
		assert.NoError(t, err)
	})

	withServer(t, handler, func(provider *dropletProviderImpl) {
		metadata, err := provider.Get(context.Background())
		assert.Equal(t, "nyc3", metadata.Region)
		assert.Nil(t, err)
	})
}

func withServer(t testing.TB, handler http.Handler, test func(*dropletProviderImpl)) {
	srv := httptest.NewServer(handler)
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		panic(err)
	}
	test(&dropletProviderImpl{
		client: dometa.NewClient(dometa.WithBaseURL(u)),
	})
}
