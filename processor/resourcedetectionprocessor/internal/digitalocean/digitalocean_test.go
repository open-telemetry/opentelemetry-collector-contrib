// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package digitalocean

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	do "github.com/digitalocean/go-metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const digitalOceanMetadataJSON = `{
  "droplet_id": 2756294,
  "hostname": "sample-droplet",
  "region": "nyc3",
  "public_keys": [],
  "interfaces": {},
  "floating_ip": {},
  "reserved_ip": {},
  "dns": {},
  "features": {}
}`

func TestNewDetector(t *testing.T) {
	// Harmless test server
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	orig := newDigitalOceanClient
	t.Cleanup(func() { newDigitalOceanClient = orig })

	base, err := url.Parse(srv.URL)
	require.NoError(t, err)

	// Force the SDK to hit our test server.
	newDigitalOceanClient = func() *do.Client {
		return do.NewClient(
			do.WithBaseURL(base),
			do.WithHTTPClient(srv.Client()),
		)
	}

	det, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	require.NotNil(t, det)
}

func TestDigitalOceanDetector_Detect_OK_JSON(t *testing.T) {
	// Serve the entire payload on /metadata/v1.json (bulk endpoint)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metadata/v1.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(digitalOceanMetadataJSON))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	orig := newDigitalOceanClient
	t.Cleanup(func() { newDigitalOceanClient = orig })

	base, err := url.Parse(srv.URL)
	require.NoError(t, err)

	newDigitalOceanClient = func() *do.Client {
		return do.NewClient(
			do.WithBaseURL(base),
			do.WithHTTPClient(srv.Client()),
		)
	}

	det, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)

	res, schemaURL, err := det.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, conventions.SchemaURL, schemaURL)

	attrs := res.Attributes().AsRaw()
	want := map[string]any{
		string(conventions.CloudProviderKey): "digitalocean",
		string(conventions.HostIDKey):        "2756294",
		string(conventions.HostNameKey):      "sample-droplet",
		string(conventions.CloudRegionKey):   "nyc3",
	}
	assert.Equal(t, want, attrs)
}

func TestDigitalOceanDetector_NotOnDigitalOcean_JSON(t *testing.T) {
	// 404 everything (including /metadata/v1.json)
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	orig := newDigitalOceanClient
	t.Cleanup(func() { newDigitalOceanClient = orig })

	base, err := url.Parse(srv.URL)
	require.NoError(t, err)

	newDigitalOceanClient = func() *do.Client {
		return do.NewClient(
			do.WithBaseURL(base),
			do.WithHTTPClient(srv.Client()),
		)
	}

	det, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)

	res, schemaURL, err := det.Detect(t.Context())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
	assert.Empty(t, schemaURL)
}
