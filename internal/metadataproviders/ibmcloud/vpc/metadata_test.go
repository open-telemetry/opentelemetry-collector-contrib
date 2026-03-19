// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testInstanceJSON is a real IBM Cloud VPC IMDS response. The provider must
// correctly extract only the fields it needs and ignore everything else.
const testInstanceJSON = `{
	"availability": {"class": "standard"},
	"availability_policy": {"host_failure": "restart", "preemption": "stop"},
	"bandwidth": 1000,
	"boot_volume_attachment": {
		"device": {"id": "02x7-3b4ca26d-365b-4616-8876-eae4014ebaa8-5mvcc"},
		"id": "02x7-3b4ca26d-365b-4616-8876-eae4014ebaa8",
		"name": "reason-foil-prong-starry",
		"volume": {
			"crn": "crn:v1:bluemix:public:is:eu-es-2:a/bab397cffebd40329900ec4b9039793a::volume:r050-1a73bf3a-789d-43bb-9066-ddbd7db6236e",
			"id": "r050-1a73bf3a-789d-43bb-9066-ddbd7db6236e",
			"name": "otel-collector-boot-1773415613000",
			"resource_type": "volume"
		}
	},
	"cluster_network_attachments": [],
	"confidential_compute_mode": "disabled",
	"created_at": "2026-03-13T15:29:48.000Z",
	"crn": "crn:v1:bluemix:public:is:eu-es-2:a/bab397cffebd40329900ec4b9039793a::instance:02x7_8340ad93-8b46-41cf-95b5-6585f54dd419",
	"disks": [],
	"enable_secure_boot": false,
	"health_reasons": [],
	"health_state": "ok",
	"id": "02x7_8340ad93-8b46-41cf-95b5-6585f54dd419",
	"image": {
		"crn": "crn:v1:bluemix:public:is:eu-es:a/811f8abfbd32425597dc7ba40da98fa6::image:r050-8844669d-ac98-4de5-8651-109645ead299",
		"id": "r050-8844669d-ac98-4de5-8651-109645ead299",
		"name": "ibm-ubuntu-24-04-4-minimal-amd64-1",
		"resource_type": "image"
	},
	"lifecycle_reasons": [],
	"lifecycle_state": "stable",
	"memory": 1,
	"metadata_service": {"enabled": true, "protocol": "https", "response_hop_limit": 1},
	"name": "otel-collector",
	"network_attachments": [
		{
			"id": "02x7-20a171f5-d487-40ba-a588-e6c8d59dc190",
			"name": "eth0",
			"primary_ip": {"address": "10.251.64.4", "id": "02x7-d0c81b79-6834-4de1-9e9d-4771d6c80665", "name": "squall-bookcases-parsnip-usable", "resource_type": "subnet_reserved_ip"},
			"resource_type": "instance_network_attachment",
			"subnet": {"crn": "crn:v1:bluemix:public:is:eu-es-2:a/bab397cffebd40329900ec4b9039793a::subnet:02x7-764266e9-0156-4a2c-9c93-bb37722376cb", "id": "02x7-764266e9-0156-4a2c-9c93-bb37722376cb", "name": "eu-es-2-default-subnet", "resource_type": "subnet"},
			"virtual_network_interface": {"crn": "crn:v1:bluemix:public:is:eu-es-2:a/bab397cffebd40329900ec4b9039793a::virtual-network-interface:02x7-a32c0941-10e5-48a6-8c0e-7af0cf265777", "id": "02x7-a32c0941-10e5-48a6-8c0e-7af0cf265777", "name": "washhouse-deputy-delicate-racer", "resource_type": "virtual_network_interface"}
		}
	],
	"network_interfaces": [
		{
			"id": "02x7-20a171f5-d487-40ba-a588-e6c8d59dc190",
			"name": "eth0",
			"primary_ipv4_address": "10.251.64.4",
			"resource_type": "network_interface",
			"subnet": {"crn": "crn:v1:bluemix:public:is:eu-es-2:a/bab397cffebd40329900ec4b9039793a::subnet:02x7-764266e9-0156-4a2c-9c93-bb37722376cb", "id": "02x7-764266e9-0156-4a2c-9c93-bb37722376cb", "name": "eu-es-2-default-subnet", "resource_type": "subnet"}
		}
	],
	"numa_count": 1,
	"primary_network_attachment": {
		"id": "02x7-20a171f5-d487-40ba-a588-e6c8d59dc190",
		"name": "eth0",
		"primary_ip": {"address": "10.251.64.4", "id": "02x7-d0c81b79-6834-4de1-9e9d-4771d6c80665", "name": "squall-bookcases-parsnip-usable", "resource_type": "subnet_reserved_ip"},
		"resource_type": "instance_network_attachment",
		"subnet": {"crn": "crn:v1:bluemix:public:is:eu-es-2:a/bab397cffebd40329900ec4b9039793a::subnet:02x7-764266e9-0156-4a2c-9c93-bb37722376cb", "id": "02x7-764266e9-0156-4a2c-9c93-bb37722376cb", "name": "eu-es-2-default-subnet", "resource_type": "subnet"},
		"virtual_network_interface": {"crn": "crn:v1:bluemix:public:is:eu-es-2:a/bab397cffebd40329900ec4b9039793a::virtual-network-interface:02x7-a32c0941-10e5-48a6-8c0e-7af0cf265777", "id": "02x7-a32c0941-10e5-48a6-8c0e-7af0cf265777", "name": "washhouse-deputy-delicate-racer", "resource_type": "virtual_network_interface"}
	},
	"primary_network_interface": {
		"id": "02x7-20a171f5-d487-40ba-a588-e6c8d59dc190",
		"name": "eth0",
		"primary_ipv4_address": "10.251.64.4",
		"resource_type": "network_interface",
		"subnet": {"crn": "crn:v1:bluemix:public:is:eu-es-2:a/bab397cffebd40329900ec4b9039793a::subnet:02x7-764266e9-0156-4a2c-9c93-bb37722376cb", "id": "02x7-764266e9-0156-4a2c-9c93-bb37722376cb", "name": "eu-es-2-default-subnet", "resource_type": "subnet"}
	},
	"profile": {"name": "nxf-1x1", "resource_type": "instance_profile"},
	"reservation_affinity": {"policy": "disabled", "pool": []},
	"resource_group": {"id": "6701a34345c54823bd156b32d4428c78", "name": "Default"},
	"resource_type": "instance",
	"startable": true,
	"status": "running",
	"status_reasons": [],
	"total_network_bandwidth": 500,
	"total_volume_bandwidth": 500,
	"vcpu": {"architecture": "amd64", "burst": {"limit": 100}, "count": 1, "manufacturer": "intel", "percentage": 100},
	"volume_attachments": [
		{
			"device": {"id": "02x7-3b4ca26d-365b-4616-8876-eae4014ebaa8-5mvcc"},
			"id": "02x7-3b4ca26d-365b-4616-8876-eae4014ebaa8",
			"name": "reason-foil-prong-starry",
			"volume": {"crn": "crn:v1:bluemix:public:is:eu-es-2:a/bab397cffebd40329900ec4b9039793a::volume:r050-1a73bf3a-789d-43bb-9066-ddbd7db6236e", "id": "r050-1a73bf3a-789d-43bb-9066-ddbd7db6236e", "name": "otel-collector-boot-1773415613000", "resource_type": "volume"}
		}
	],
	"volume_bandwidth_qos_mode": "pooled",
	"vpc": {
		"crn": "crn:v1:bluemix:public:is:eu-es:a/bab397cffebd40329900ec4b9039793a::vpc:r050-bb31bf47-5a58-407e-b671-05a97def748e",
		"id": "r050-bb31bf47-5a58-407e-b671-05a97def748e",
		"name": "eu-es-default-vpc-03131529",
		"resource_type": "vpc"
	},
	"zone": {"name": "eu-es-2"}
}`

// writeJSON encodes v as JSON to w, failing the test on error.
func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(v))
}

// writeBody writes raw bytes to w, failing the test on error.
func writeBody(t *testing.T, w http.ResponseWriter, data []byte) {
	t.Helper()
	_, err := w.Write(data)
	require.NoError(t, err)
}

func newTestServer(t *testing.T, tokenCount *atomic.Int32) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == tokenPath && r.Method == http.MethodPut:
			if r.Header.Get(metadataFlavorKey) != metadataFlavorValue {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if tokenCount != nil {
				tokenCount.Add(1)
			}
			resp := tokenResponse{
				AccessToken: "test-token",
				ExpiresIn:   300,
			}
			writeJSON(t, w, resp)

		case r.URL.Path == instancePath && r.Method == http.MethodGet:
			auth := r.Header.Get("Authorization")
			if auth != "Bearer test-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			writeBody(t, w, []byte(testInstanceJSON))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestInstanceMetadata(t *testing.T) {
	server := newTestServer(t, nil)
	defer server.Close()

	provider := newProvider(server.URL)
	meta, err := provider.InstanceMetadata(t.Context())

	require.NoError(t, err)
	require.NotNil(t, meta)
	require.Equal(t, "02x7_8340ad93-8b46-41cf-95b5-6585f54dd419", meta.ID)
	require.Equal(t, "crn:v1:bluemix:public:is:eu-es-2:a/bab397cffebd40329900ec4b9039793a::instance:02x7_8340ad93-8b46-41cf-95b5-6585f54dd419", meta.CRN)
	require.Equal(t, "otel-collector", meta.Name)
	require.Equal(t, "nxf-1x1", meta.Profile.Name)
	require.Equal(t, "eu-es-2", meta.Zone.Name)
	require.Equal(t, "eu-es-default-vpc-03131529", meta.VPC.Name)
	require.Equal(t, "6701a34345c54823bd156b32d4428c78", meta.ResourceGroup.ID)
	require.Equal(t, "ibm-ubuntu-24-04-4-minimal-amd64-1", meta.Image.Name)
}

func TestTokenCaching(t *testing.T) {
	var tokenCount atomic.Int32
	server := newTestServer(t, &tokenCount)
	defer server.Close()

	provider := newProvider(server.URL)

	// First call should acquire a token
	_, err := provider.InstanceMetadata(t.Context())
	require.NoError(t, err)
	require.Equal(t, int32(1), tokenCount.Load())

	// Second call should use the cached token
	_, err = provider.InstanceMetadata(t.Context())
	require.NoError(t, err)
	require.Equal(t, int32(1), tokenCount.Load())
}

func TestTokenRefreshOnExpiry(t *testing.T) {
	var tokenCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case tokenPath:
			tokenCount.Add(1)
			resp := tokenResponse{
				AccessToken: "test-token",
				ExpiresIn:   1, // 1 second TTL
			}
			writeJSON(t, w, resp)

		case instancePath:
			w.Header().Set("Content-Type", "application/json")
			writeBody(t, w, []byte(testInstanceJSON))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := newProvider(server.URL)

	// First call acquires a token
	_, err := provider.InstanceMetadata(t.Context())
	require.NoError(t, err)
	require.Equal(t, int32(1), tokenCount.Load())

	// With 1s TTL and 30s buffer, the token should already be expired (negative expiry).
	// Second call should acquire a new token.
	_, err = provider.InstanceMetadata(t.Context())
	require.NoError(t, err)
	require.Equal(t, int32(2), tokenCount.Load())
}

func TestTokenAcquisitionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			w.WriteHeader(http.StatusServiceUnavailable)
			writeBody(t, w, []byte("service unavailable"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := newProvider(server.URL)
	meta, err := provider.InstanceMetadata(t.Context())

	require.Error(t, err)
	require.Nil(t, meta)
	require.Contains(t, err.Error(), "token request returned 503")
}

func TestMetadataRetrievalFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case tokenPath:
			resp := tokenResponse{AccessToken: "test-token", ExpiresIn: 300}
			writeJSON(t, w, resp)

		case instancePath:
			w.WriteHeader(http.StatusInternalServerError)
			writeBody(t, w, []byte("internal error"))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := newProvider(server.URL)
	meta, err := provider.InstanceMetadata(t.Context())

	require.Error(t, err)
	require.Nil(t, meta)
	require.Contains(t, err.Error(), "instance metadata request returned 500")
}

func TestMalformedTokenResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			w.Header().Set("Content-Type", "application/json")
			writeBody(t, w, []byte("not json"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	provider := newProvider(server.URL)
	meta, err := provider.InstanceMetadata(t.Context())

	require.Error(t, err)
	require.Nil(t, meta)
	require.Contains(t, err.Error(), "failed to decode token response")
}

func TestMalformedMetadataResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case tokenPath:
			resp := tokenResponse{AccessToken: "test-token", ExpiresIn: 300}
			writeJSON(t, w, resp)

		case instancePath:
			w.Header().Set("Content-Type", "application/json")
			writeBody(t, w, []byte("not json"))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := newProvider(server.URL)
	meta, err := provider.InstanceMetadata(t.Context())

	require.Error(t, err)
	require.Nil(t, meta)
	require.Contains(t, err.Error(), "failed to decode instance metadata")
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newProvider(server.URL)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	meta, err := provider.InstanceMetadata(ctx)

	require.Error(t, err)
	require.Nil(t, meta)
}

func TestHTTPEndpoint(t *testing.T) {
	provider := NewProvider("http")
	mc := provider.(*metadataClient)
	require.Equal(t, "http://"+metadataHost, mc.endpoint)
}

func TestHTTPSEndpoint(t *testing.T) {
	provider := NewProvider("https")
	mc := provider.(*metadataClient)
	require.Equal(t, "https://"+metadataHost, mc.endpoint)
}

func TestMissingMetadataFlavorHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == tokenPath {
			if r.Header.Get(metadataFlavorKey) != metadataFlavorValue {
				w.WriteHeader(http.StatusBadRequest)
				writeBody(t, w, []byte("Missing Metadata-Flavor header"))
				return
			}
			resp := tokenResponse{AccessToken: "test-token", ExpiresIn: 300}
			writeJSON(t, w, resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Verify the server rejects requests without the header by using a raw client.
	// The provider always sends the header, so the actual test is that NewProvider
	// correctly sets it. We verify this indirectly through TestInstanceMetadata success.
	provider := newProvider(server.URL)
	_, err := provider.InstanceMetadata(t.Context())
	// This will fail at metadata step since we don't serve instance metadata,
	// but the token step should succeed, proving the header is set.
	require.Error(t, err)
	require.NotContains(t, err.Error(), "token request returned 400")
}
