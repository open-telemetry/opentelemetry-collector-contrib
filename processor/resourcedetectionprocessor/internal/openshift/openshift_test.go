// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openshift // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift"

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift/internal/metadata"
)

const infrastructurePath = "/apis/config.openshift.io/v1/infrastructures/cluster/status"

const (
	awsInfra       = `{"status":{"infrastructureName":"my-cluster","platformStatus":{"type":"AWS","aws":{"region":"US-EAST-1"}}}}`
	azureInfra     = `{"status":{"infrastructureName":"my-cluster","platformStatus":{"type":"Azure","azure":{"cloudName":"AzurePublicCloud"}}}}`
	openstackInfra = `{"status":{"infrastructureName":"my-cluster","platformStatus":{"type":"OpenStack","openstack":{"cloudName":"openstack"}}}}`
)

// setRemoveCloudNameRegionGate forces the state of the removeCloudNameRegion gate
// for the duration of the test.
func setRemoveCloudNameRegionGate(t *testing.T, enabled bool) {
	gate := metadata.ProcessorResourcedetectionOpenshiftRemoveCloudNameRegionFeatureGate
	originalValue := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), originalValue))
	})
}

// newInfraServer serves body on the Infrastructure status endpoint with the given status code.
func newInfraServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != infrastructurePath {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// testConfig points the detector at srv. A plain HTTP address needs no certificate
// authority, and an explicit token skips the projected service account token.
func testConfig(srv *httptest.Server) Config {
	cfg := CreateDefaultConfig()
	cfg.Address = srv.URL
	cfg.Token = "token"
	return cfg
}

func detect(t *testing.T, cfg Config, failOnMissingMetadata bool) (map[string]any, error) {
	t.Helper()
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, failOnMissingMetadata)
	require.NoError(t, err)
	res, _, err := d.Detect(t.Context())
	return res.Attributes().AsRaw(), err
}

func TestDetect(t *testing.T) {
	setRemoveCloudNameRegionGate(t, true)
	tests := []struct {
		name string
		body string
		want map[string]any
	}{
		{
			name: "aws",
			body: awsInfra,
			want: map[string]any{
				"k8s.cluster.name": "my-cluster",
				"cloud.provider":   "aws",
				"cloud.platform":   "aws_openshift",
				"cloud.region":     "us-east-1",
			},
		},
		{
			name: "azure reports no region",
			body: azureInfra,
			want: map[string]any{
				"k8s.cluster.name": "my-cluster",
				"cloud.provider":   "azure",
				"cloud.platform":   "azure.openshift",
			},
		},
		{
			name: "openstack reports no cloud attributes",
			body: openstackInfra,
			want: map[string]any{
				"k8s.cluster.name": "my-cluster",
			},
		},
		{
			name: "partial result keeps what was detected",
			body: `{"status":{"platformStatus":{"type":"GCP","gcp":{"region":"europe-west1"}}}}`,
			want: map[string]any{
				"cloud.provider": "gcp",
				"cloud.platform": "gcp_openshift",
				"cloud.region":   "europe-west1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := detect(t, testConfig(newInfraServer(t, http.StatusOK, tt.body)), true)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectLegacyCloudNameRegion(t *testing.T) {
	setRemoveCloudNameRegionGate(t, false)
	tests := []struct {
		name string
		body string
		want map[string]any
	}{
		{
			name: "azure",
			body: azureInfra,
			want: map[string]any{
				"k8s.cluster.name": "my-cluster",
				"cloud.provider":   "azure",
				"cloud.platform":   "azure.openshift",
				"cloud.region":     "azurepubliccloud",
			},
		},
		{
			name: "openstack",
			body: openstackInfra,
			want: map[string]any{
				"k8s.cluster.name": "my-cluster",
				"cloud.region":     "openstack",
			},
		},
		{
			name: "aws keeps the sdk region",
			body: awsInfra,
			want: map[string]any{
				"k8s.cluster.name": "my-cluster",
				"cloud.provider":   "aws",
				"cloud.platform":   "aws_openshift",
				"cloud.region":     "us-east-1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := detect(t, testConfig(newInfraServer(t, http.StatusOK, tt.body)), true)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("disabled attribute", func(t *testing.T) {
		cfg := testConfig(newInfraServer(t, http.StatusOK, azureInfra))
		cfg.ResourceAttributes.CloudRegion.Enabled = false
		got, err := detect(t, cfg, true)
		require.NoError(t, err)
		assert.NotContains(t, got, "cloud.region")
	})
}

func TestDetectFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		// A plain Kubernetes API server does not serve the OpenShift config API.
		{name: "not openshift", status: http.StatusNotFound},
		{name: "forbidden", status: http.StatusForbidden},
		{name: "server error", status: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newInfraServer(t, tt.status, "")

			got, err := detect(t, testConfig(srv), false)
			require.NoError(t, err)
			assert.Empty(t, got)

			got, err = detect(t, testConfig(srv), true)
			assert.Error(t, err)
			assert.Empty(t, got)
		})
	}
}

func TestDetectDisabledAttributes(t *testing.T) {
	cfg := testConfig(newInfraServer(t, http.StatusOK, awsInfra))
	cfg.ResourceAttributes.CloudRegion.Enabled = false
	cfg.ResourceAttributes.K8sClusterName.Enabled = false

	got, err := detect(t, cfg, true)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"cloud.provider": "aws",
		"cloud.platform": "aws_openshift",
	}, got)
}

func TestDetectWithTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(awsInfra))
	}))
	t.Cleanup(srv.Close)

	cfg := testConfig(srv)
	cfg.TLSs.CAPem = configopaque.String(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))

	got, err := detect(t, cfg, true)
	require.NoError(t, err)
	assert.Equal(t, "my-cluster", got["k8s.cluster.name"])
}

func TestNewDetectorInvalidTLS(t *testing.T) {
	cfg := CreateDefaultConfig()
	cfg.TLSs.CAFile = "/does/not/exist"
	_, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	assert.Error(t, err)
}

func TestDetectNotInCluster(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	got, err := detect(t, CreateDefaultConfig(), false)
	require.NoError(t, err)
	assert.Empty(t, got)

	_, err = detect(t, CreateDefaultConfig(), true)
	assert.Error(t, err)
}
