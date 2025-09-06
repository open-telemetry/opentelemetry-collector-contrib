// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hetzner

import (
	"net/http"
	"net/http/httptest"
	"testing"

	hcloudmeta "github.com/hetznercloud/hcloud-go/v2/hcloud/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func withFakeMetaServer(t *testing.T, mux http.Handler) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	orig := newHcloudClient
	newHcloudClient = func() *hcloudmeta.Client {
		return hcloudmeta.NewClient(hcloudmeta.WithEndpoint(srv.URL))
	}
	t.Cleanup(func() { newHcloudClient = orig })
}

func TestNewDetector(t *testing.T) {
	dcfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), dcfg)
	require.NoError(t, err)
	assert.NotNil(t, d)
}

func TestHetznerDetector_Detect_OK(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/hostname", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("srv-123")) })
	mux.HandleFunc("/instance-id", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("987654321")) })
	mux.HandleFunc("/region", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("nbg1")) })
	mux.HandleFunc("/availability-zone", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("nbg1-dc3")) })
	withFakeMetaServer(t, mux)

	cfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, conventions.SchemaURL, schemaURL)

	want := map[string]any{
		string(conventions.CloudProviderKey):         TypeStr,
		string(conventions.HostIDKey):                "987654321",
		string(conventions.HostNameKey):              "srv-123",
		string(conventions.CloudRegionKey):           "nbg1",
		string(conventions.CloudAvailabilityZoneKey): "nbg1-dc3",
	}
	assert.Equal(t, want, res.Attributes().AsRaw())
}

func TestHetznerDetector_NotOnHetzner(t *testing.T) {
	// Set up a server so that IsHcloudServer() returns false (Hostname() errors).
	mux := http.NewServeMux()
	mux.HandleFunc("/hostname", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// Temporarily point our detector’s client factory to the test server.
	orig := newHcloudClient
	newHcloudClient = func() *hcloudmeta.Client {
		return hcloudmeta.NewClient(hcloudmeta.WithEndpoint(ts.URL))
	}
	t.Cleanup(func() { newHcloudClient = orig })

	cfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
	assert.Empty(t, schemaURL)
}

func TestHetznerDetector_HostnameError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/hostname", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	withFakeMetaServer(t, mux)

	cfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
	assert.Empty(t, schemaURL)
}
