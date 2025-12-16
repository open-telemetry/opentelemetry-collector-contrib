// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scaleway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	instance "github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const scalewaySample = `{
  "allowed_actions": ["poweroff","terminate","reboot","stop_in_place","backup"],
  "arch": "x86_64",
  "boot_type": "local",
  "bootscript": null,
  "commercial_type": "STARDUST1-S",
  "creation_date": "2025-09-12T22:42:24.690284+00:00",
  "dynamic_ip_required": false,
  "enable_ipv6": false,
  "end_of_service": false,
  "extra_networks": [],
  "filesystems": [],
  "hostname": "scw-interesting-bassi",
  "id": "daa2ea5a-0ee6-4cdc-9f1a-e0d1cb4e6d86",
  "image": {
    "arch": "x86_64",
    "creation_date": "2025-09-12T09:03:25.327625+00:00",
    "default_bootscript": null,
    "extra_volumes": {},
    "from_server": "",
    "id": "01fe25a9-2e95-41e2-9f3f-f7d3a00604be",
    "modification_date": "2025-09-12T09:03:25.327625+00:00",
    "name": "Ubuntu 24.04 Noble Numbat",
    "organization": "51b656e3-4865-41e8-adbc-0c45bdd780db",
    "project": "51b656e3-4865-41e8-adbc-0c45bdd780db",
    "public": true,
    "root_volume": {
      "id": "4f312959-ef1f-41c9-9a98-a90f800e09c3",
      "name": "",
      "size": 0,
      "volume_type": "sbs_snapshot"
    },
    "state": "available",
    "tags": [],
    "zone": "nl-ams-1"
  },
  "ipv6": null,
  "location": {
    "cluster_id": "15",
    "hypervisor_id": "402",
    "node_id": "134",
    "platform_id": "23",
    "zone_id": "nl-ams-1"
  },
  "mac_address": "de:00:00:58:4f:8f",
  "maintenances": [],
  "modification_date": "2025-09-12T22:42:28.448040+00:00",
  "name": "scw-interesting-bassi",
  "organization": "10542306-0c75-4265-9c2c-1fbfe4ea0bf0",
  "placement_group": null,
  "private_ip": null,
  "private_nics": [],
  "project": "10542306-0c75-4265-9c2c-1fbfe4ea0bf0",
  "protected": false,
  "public_ip": {
    "address": "51.15.68.77",
    "dynamic": false,
    "family": "inet",
    "gateway": "62.210.0.1",
    "id": "8eb23745-d030-4da5-b0eb-75f2f2ccd9c1",
    "ipam_id": "8dcfbb2c-83fb-4403-8bf0-8eae316b4be3",
    "netmask": "32",
    "provisioning_mode": "dhcp",
    "state": "attached",
    "tags": []
  },
  "public_ips": [
    {
      "address": "51.15.68.77",
      "dynamic": false,
      "family": "inet",
      "gateway": "62.210.0.1",
      "id": "8eb23745-d030-4da5-b0eb-75f2f2ccd9c1",
      "ipam_id": "8dcfbb2c-83fb-4403-8bf0-8eae316b4be3",
      "netmask": "32",
      "provisioning_mode": "dhcp",
      "state": "attached",
      "tags": []
    }
  ],
  "public_ips_v4": [
    {
      "address": "51.15.68.77",
      "dynamic": false,
      "family": "inet",
      "gateway": "62.210.0.1",
      "id": "8eb23745-d030-4da5-b0eb-75f2f2ccd9c1",
      "ipam_id": "8dcfbb2c-83fb-4403-8bf0-8eae316b4be3",
      "netmask": "32",
      "provisioning_mode": "dhcp",
      "state": "attached",
      "tags": []
    }
  ],
  "public_ips_v6": [],
  "routed_ip_enabled": true,
  "security_group": {
    "id": "6dfa8805-0a86-42e8-b781-2889e726bf73",
    "name": "Default security group"
  },
  "ssh_public_keys": [],
  "state": "running",
  "state_detail": "booted",
  "tags": [],
  "volumes": {
    "0": {
      "boot": false,
      "id": "76d8ae14-6a36-43b3-b0e3-bce79cbd7b06",
      "volume_type": "sbs_volume",
      "zone": "nl-ams-1"
    }
  },
  "zone": "nl-ams-1"
}`

func TestNewDetector(t *testing.T) {
	// Use a real SDK client pointed at a harmless test server.
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	orig := newScalewayClient
	t.Cleanup(func() { newScalewayClient = orig })

	// Force the SDK to hit our test server instead of 169.254.42.42
	newScalewayClient = func() *instance.MetadataAPI {
		api := instance.NewMetadataAPI()
		api.MetadataURL = &srv.URL
		return api
	}

	det, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	require.NotNil(t, det)
}

func TestScalewayDetector_Detect_OK(t *testing.T) {
	// Serve exactly your sample on /conf?format=json
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/conf") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(scalewaySample))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	orig := newScalewayClient
	t.Cleanup(func() { newScalewayClient = orig })
	newScalewayClient = func() *instance.MetadataAPI {
		api := instance.NewMetadataAPI()
		api.MetadataURL = &srv.URL
		return api
	}

	det, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)

	res, schemaURL, err := det.Detect(t.Context())
	require.NoError(t, err)
	require.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")

	attrs := res.Attributes().AsRaw()
	// Region is derived from zone by trimming the trailing segment: "nl-ams-1" -> "nl-ams"
	want := map[string]any{
		"cloud.provider":          "scaleway_cloud",
		"cloud.platform":          "scaleway_cloud_compute",
		"cloud.account.id":        "10542306-0c75-4265-9c2c-1fbfe4ea0bf0",
		"cloud.availability_zone": "nl-ams-1",
		"cloud.region":            "nl-ams",
		"host.id":                 "daa2ea5a-0ee6-4cdc-9f1a-e0d1cb4e6d86",
		"host.image.id":           "01fe25a9-2e95-41e2-9f3f-f7d3a00604be",
		"host.image.name":         "Ubuntu 24.04 Noble Numbat",
		"host.name":               "scw-interesting-bassi",
		"host.type":               "STARDUST1-S",
	}
	assert.Equal(t, want, attrs)
}

func TestScalewayDetector_NotOnScaleway(t *testing.T) {
	// 404 everything to simulate "metadata not reachable"
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	orig := newScalewayClient
	t.Cleanup(func() { newScalewayClient = orig })
	newScalewayClient = func() *instance.MetadataAPI {
		api := instance.NewMetadataAPI()
		api.MetadataURL = &srv.URL
		return api
	}

	det, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)

	res, schemaURL, err := det.Detect(t.Context())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
	assert.Empty(t, schemaURL)
}
