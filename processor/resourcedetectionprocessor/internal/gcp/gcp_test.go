// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"

	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	localMetadata "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp/internal/metadata"
)

const testSchemaURL = "https://opentelemetry.io/schemas/1.40.0"

type mockInstancesClient struct {
	labels map[string]string
	err    error
}

func (m *mockInstancesClient) Get(_ context.Context, _ *computepb.GetInstanceRequest) (*computepb.Instance, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &computepb.Instance{Labels: m.labels}, nil
}

func (*mockInstancesClient) Close() error { return nil }

type mockInstancesBuilder struct {
	client instancesAPI
	err    error
}

func (b *mockInstancesBuilder) buildClient(_ context.Context) (instancesAPI, error) {
	return b.client, b.err
}

func mustRe(p string) *regexp.Regexp {
	r, err := regexp.Compile(p)
	if err != nil {
		panic(err)
	}
	return r
}

type fakeDetector struct {
	res *sdkresource.Resource
	err error
}

func (f fakeDetector) Detect(context.Context) (*sdkresource.Resource, error) {
	return f.res, f.err
}

func withFakeDetector(t *testing.T, res *sdkresource.Resource, err error) {
	t.Helper()
	orig := newResourceDetector
	newResourceDetector = func() sdkresource.Detector {
		return fakeDetector{res: res, err: err}
	}
	t.Cleanup(func() { newResourceDetector = orig })
}

func TestNewDetector(t *testing.T) {
	cfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)
	require.NotNil(t, d)
}

func TestDetect(t *testing.T) {
	for _, tc := range []struct {
		desc             string
		sdkResource      *sdkresource.Resource
		sdkErr           error
		cfgModifier      func(*localMetadata.ResourceAttributesConfig)
		expectErr        bool
		expectedResource map[string]any
	}{
		{
			desc: "zonal GKE cluster",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "my-project"),
				attribute.String("cloud.platform", "gcp_kubernetes_engine"),
				attribute.String("k8s.cluster.name", "my-cluster"),
				attribute.String("cloud.availability_zone", "us-central1-c"),
				attribute.String("host.id", "1472385723456792345"),
				attribute.String("host.name", "my-gke-node-1234"),
			),
			expectedResource: map[string]any{
				"cloud.provider":          "gcp",
				"cloud.account.id":        "my-project",
				"cloud.platform":          "gcp_kubernetes_engine",
				"k8s.cluster.name":        "my-cluster",
				"cloud.availability_zone": "us-central1-c",
				"host.id":                 "1472385723456792345",
				"host.name":               "my-gke-node-1234",
			},
		},
		{
			desc: "regional GKE cluster",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "my-project"),
				attribute.String("cloud.platform", "gcp_kubernetes_engine"),
				attribute.String("k8s.cluster.name", "my-cluster"),
				attribute.String("cloud.region", "us-central1"),
				attribute.String("host.id", "1472385723456792345"),
				attribute.String("host.name", "my-gke-node-1234"),
			),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
				"cloud.platform":   "gcp_kubernetes_engine",
				"k8s.cluster.name": "my-cluster",
				"cloud.region":     "us-central1",
				"host.id":          "1472385723456792345",
				"host.name":        "my-gke-node-1234",
			},
		},
		{
			desc: "regional GKE cluster with workload identity",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "my-project"),
				attribute.String("cloud.platform", "gcp_kubernetes_engine"),
				attribute.String("k8s.cluster.name", "my-cluster"),
				attribute.String("cloud.region", "us-central1"),
				attribute.String("host.id", "1472385723456792345"),
			),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
				"cloud.platform":   "gcp_kubernetes_engine",
				"k8s.cluster.name": "my-cluster",
				"cloud.region":     "us-central1",
				"host.id":          "1472385723456792345",
			},
		},
		{
			desc: "GCE",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "my-project"),
				attribute.String("cloud.platform", "gcp_compute_engine"),
				attribute.String("host.id", "1472385723456792345"),
				attribute.String("host.name", "my-gke-node-1234"),
				attribute.String("host.type", "n1-standard1"),
				attribute.String("cloud.region", "us-central1"),
				attribute.String("cloud.availability_zone", "us-central1-c"),
				attribute.String("gcp.gce.instance.name", "my-gke-node-1234"),
				attribute.String("gcp.gce.instance.hostname", "custom.dns.example.com"),
			),
			expectedResource: map[string]any{
				"cloud.provider":          "gcp",
				"cloud.account.id":        "my-project",
				"cloud.platform":          "gcp_compute_engine",
				"host.id":                 "1472385723456792345",
				"host.name":               "my-gke-node-1234",
				"host.type":               "n1-standard1",
				"cloud.region":            "us-central1",
				"cloud.availability_zone": "us-central1-c",
			},
		},
		{
			desc: "GCE with instance.hostname and instance.name enabled",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "my-project"),
				attribute.String("cloud.platform", "gcp_compute_engine"),
				attribute.String("host.id", "1472385723456792345"),
				attribute.String("host.name", "my-gke-node-1234"),
				attribute.String("host.type", "n1-standard1"),
				attribute.String("cloud.region", "us-central1"),
				attribute.String("cloud.availability_zone", "us-central1-c"),
				attribute.String("gcp.gce.instance.name", "my-gke-node-1234"),
				attribute.String("gcp.gce.instance.hostname", "custom.dns.example.com"),
			),
			cfgModifier: func(cfg *localMetadata.ResourceAttributesConfig) {
				cfg.GcpGceInstanceHostname.Enabled = true
				cfg.GcpGceInstanceName.Enabled = true
			},
			expectedResource: map[string]any{
				"cloud.provider":            "gcp",
				"cloud.account.id":          "my-project",
				"cloud.platform":            "gcp_compute_engine",
				"host.id":                   "1472385723456792345",
				"host.name":                 "my-gke-node-1234",
				"host.type":                 "n1-standard1",
				"cloud.region":              "us-central1",
				"cloud.availability_zone":   "us-central1-c",
				"gcp.gce.instance.hostname": "custom.dns.example.com",
				"gcp.gce.instance.name":     "my-gke-node-1234",
			},
		},
		{
			desc: "GCE with MIG",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "my-project"),
				attribute.String("cloud.platform", "gcp_compute_engine"),
				attribute.String("host.id", "1472385723456792345"),
				attribute.String("host.name", "my-gke-node-1234"),
				attribute.String("host.type", "n1-standard1"),
				attribute.String("cloud.region", "us-central1"),
				attribute.String("cloud.availability_zone", "us-central1-c"),
				attribute.String("gcp.gce.instance.name", "my-gke-node-1234"),
				attribute.String("gcp.gce.instance.hostname", "custom.dns.example.com"),
				attribute.String("gcp.gce.instance_group_manager.name", "my-gke-node"),
				attribute.String("gcp.gce.instance_group_manager.region", "us-central1"),
			),
			expectedResource: map[string]any{
				"cloud.provider":                        "gcp",
				"cloud.account.id":                      "my-project",
				"cloud.platform":                        "gcp_compute_engine",
				"host.id":                               "1472385723456792345",
				"host.name":                             "my-gke-node-1234",
				"host.type":                             "n1-standard1",
				"cloud.region":                          "us-central1",
				"cloud.availability_zone":               "us-central1-c",
				"gcp.gce.instance_group_manager.name":   "my-gke-node",
				"gcp.gce.instance_group_manager.region": "us-central1",
			},
		},
		{
			desc: "Cloud Run",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "my-project"),
				attribute.String("cloud.platform", "gcp_cloud_run"),
				attribute.String("cloud.region", "us-central1"),
				attribute.String("faas.name", "my-service"),
				attribute.String("faas.version", "123456"),
				attribute.String("faas.instance", "1472385723456792345"),
			),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
				"cloud.platform":   "gcp_cloud_run",
				"cloud.region":     "us-central1",
				"faas.name":        "my-service",
				"faas.version":     "123456",
				"faas.instance":    "1472385723456792345",
			},
		},
		{
			desc: "Cloud Run Job",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "my-project"),
				attribute.String("cloud.platform", "gcp_cloud_run"),
				attribute.String("cloud.region", "us-central1"),
				attribute.String("faas.name", "my-service"),
				attribute.String("faas.instance", "1472385723456792345"),
				attribute.String("gcp.cloud_run.job.execution", "my-service-ajg89"),
				attribute.Int("gcp.cloud_run.job.task_index", 2),
			),
			expectedResource: map[string]any{
				"cloud.provider":               "gcp",
				"cloud.account.id":             "my-project",
				"cloud.platform":               "gcp_cloud_run",
				"cloud.region":                 "us-central1",
				"faas.name":                    "my-service",
				"faas.instance":                "1472385723456792345",
				"gcp.cloud_run.job.execution":  "my-service-ajg89",
				"gcp.cloud_run.job.task_index": "2",
			},
		},
		{
			desc: "Cloud Functions",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "my-project"),
				attribute.String("cloud.platform", "gcp_cloud_functions"),
				attribute.String("cloud.region", "us-central1"),
				attribute.String("faas.name", "my-service"),
				attribute.String("faas.version", "123456"),
				attribute.String("faas.instance", "1472385723456792345"),
			),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
				"cloud.platform":   "gcp_cloud_functions",
				"cloud.region":     "us-central1",
				"faas.name":        "my-service",
				"faas.version":     "123456",
				"faas.instance":    "1472385723456792345",
			},
		},
		{
			desc: "App Engine Standard",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "my-project"),
				attribute.String("cloud.platform", "gcp_app_engine"),
				attribute.String("cloud.region", "us-central1"),
				attribute.String("cloud.availability_zone", "us-central1-c"),
				attribute.String("faas.name", "my-service"),
				attribute.String("faas.version", "123456"),
				attribute.String("faas.instance", "1472385723456792345"),
			),
			expectedResource: map[string]any{
				"cloud.provider":          "gcp",
				"cloud.account.id":        "my-project",
				"cloud.platform":          "gcp_app_engine",
				"cloud.region":            "us-central1",
				"cloud.availability_zone": "us-central1-c",
				"faas.name":               "my-service",
				"faas.version":            "123456",
				"faas.instance":           "1472385723456792345",
			},
		},
		{
			desc: "App Engine Flex",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "my-project"),
				attribute.String("cloud.platform", "gcp_app_engine"),
				attribute.String("cloud.region", "us-central1"),
				attribute.String("cloud.availability_zone", "us-central1-c"),
				attribute.String("faas.name", "my-service"),
				attribute.String("faas.version", "123456"),
				attribute.String("faas.instance", "1472385723456792345"),
			),
			expectedResource: map[string]any{
				"cloud.provider":          "gcp",
				"cloud.account.id":        "my-project",
				"cloud.platform":          "gcp_app_engine",
				"cloud.region":            "us-central1",
				"cloud.availability_zone": "us-central1-c",
				"faas.name":               "my-service",
				"faas.version":            "123456",
				"faas.instance":           "1472385723456792345",
			},
		},
		{
			desc: "Bare Metal Solution",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.platform", "gcp_bare_metal_solution"),
				attribute.String("cloud.account.id", "my-project"),
				attribute.String("host.name", "1472385723456792345"),
				attribute.String("cloud.region", "us-central1"),
			),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
				"cloud.platform":   "gcp_bare_metal_solution",
				"cloud.region":     "us-central1",
				"host.name":        "1472385723456792345",
			},
		},
		{
			desc: "Unknown Platform",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "my-project"),
			),
			expectedResource: map[string]any{
				"cloud.provider":   "gcp",
				"cloud.account.id": "my-project",
			},
		},
		{
			desc: "error with partial resource",
			sdkResource: sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
			),
			sdkErr: fmt.Errorf("%w: failed to get metadata", sdkresource.ErrPartialResource),
			expectedResource: map[string]any{
				"cloud.provider": "gcp",
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			withFakeDetector(t, tc.sdkResource, tc.sdkErr)

			cfg := CreateDefaultConfig()
			if tc.cfgModifier != nil {
				tc.cfgModifier(&cfg.ResourceAttributes)
			}

			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
			require.NoError(t, err)

			res, schema, err := d.Detect(t.Context())
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Contains(t, schema, "https://opentelemetry.io/schemas/")
			assert.Equal(t, tc.expectedResource, res.Attributes().AsRaw(), "Resource object returned is incorrect")
		})
	}
}

func TestDetectNotOnGCP(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schema, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schema)
}

func TestDetectFailOnMissingMetadata(t *testing.T) {
	for _, tc := range []struct {
		desc        string
		sdkResource *sdkresource.Resource
		sdkErr      error
		errContains string
	}{
		{
			desc:        "sdk returns error",
			sdkResource: nil,
			sdkErr:      errors.New("failed to get metadata"),
			errContains: "gcp metadata unavailable: failed to get metadata",
		},
		{
			desc:        "sdk returns empty resource without error",
			sdkResource: sdkresource.Empty(),
			sdkErr:      nil,
			errContains: "gcp metadata unavailable",
		},
		{
			desc:        "sdk returns nil resource without error",
			sdkResource: nil,
			sdkErr:      nil,
			errContains: "gcp metadata unavailable",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			withFakeDetector(t, tc.sdkResource, tc.sdkErr)

			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
			require.NoError(t, err)

			res, schema, err := d.Detect(t.Context())
			require.Error(t, err)
			assert.ErrorContains(t, err, tc.errContains)
			assert.Empty(t, schema)
			assert.Equal(t, 0, res.Attributes().Len())
		})
	}
}

func TestGCELabels(t *testing.T) {
	tests := []struct {
		name            string
		instanceLabels  map[string]string
		instanceErr     error
		builderErr      error
		labelRegexes    []*regexp.Regexp
		expectedPresent map[string]string
		expectedAbsent  []string
	}{
		{
			name: "success case two labels matched",
			instanceLabels: map[string]string{
				"tag1":  "val1",
				"tag2":  "val2",
				"other": "nope",
			},
			labelRegexes: []*regexp.Regexp{mustRe("^tag1$"), mustRe("^tag2$")},
			expectedPresent: map[string]string{
				"tag1": "val1",
				"tag2": "val2",
			},
			expectedAbsent: []string{"other"},
		},
		{
			name:            "error case in Get",
			instanceErr:     errors.New("compute API is not available"),
			labelRegexes:    []*regexp.Regexp{mustRe("^tag1$")},
			expectedPresent: map[string]string{},
			expectedAbsent:  []string{"tag1"},
		},
		{
			name:            "buildClient error",
			builderErr:      errors.New("failed to create compute client"),
			labelRegexes:    []*regexp.Regexp{mustRe("^tag1$")},
			expectedPresent: map[string]string{},
			expectedAbsent:  []string{"tag1"},
		},
		{
			name: "no labels match regexes",
			instanceLabels: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
			labelRegexes:    []*regexp.Regexp{mustRe("^nomatch$")},
			expectedPresent: map[string]string{},
			expectedAbsent:  []string{"foo", "baz"},
		},
		{
			name: "wildcard regex matches multiple",
			instanceLabels: map[string]string{
				"env_prod": "1",
				"env_dev":  "0",
				"other":    "x",
			},
			labelRegexes: []*regexp.Regexp{mustRe("^env_")},
			expectedPresent: map[string]string{
				"env_prod": "1",
				"env_dev":  "0",
			},
			expectedAbsent: []string{"other"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gceResource := sdkresource.NewWithAttributes(
				testSchemaURL,
				attribute.String("cloud.provider", "gcp"),
				attribute.String("cloud.account.id", "test-proj"),
				attribute.String("cloud.platform", "gcp_compute_engine"),
				attribute.String("cloud.availability_zone", "us-central1-a"),
				attribute.String("cloud.region", "us-central1"),
				attribute.String("host.id", "1234567890"),
				attribute.String("host.name", "test-vm"),
				attribute.String("host.type", "n2-standard-2"),
				attribute.String("gcp.gce.instance.name", "test-vm"),
			)
			withFakeDetector(t, gceResource, nil)

			cfg := CreateDefaultConfig()
			cfg.Labels = make([]string, len(tt.labelRegexes))
			for i, r := range tt.labelRegexes {
				cfg.Labels[i] = r.String()
			}

			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
			require.NoError(t, err)

			d.(*Detector).gceClientBuilder = &mockInstancesBuilder{
				client: &mockInstancesClient{
					labels: tt.instanceLabels,
					err:    tt.instanceErr,
				},
				err: tt.builderErr,
			}

			res, _, err := d.Detect(t.Context())
			assert.NoError(t, err)

			attrs := res.Attributes()
			for k, v := range tt.expectedPresent {
				val, ok := attrs.Get(gceLabelPrefix + k)
				assert.True(t, ok, "expected %s to be present", k)
				assert.Equal(t, v, val.Str())
			}
			for _, k := range tt.expectedAbsent {
				_, ok := attrs.Get(gceLabelPrefix + k)
				assert.False(t, ok, "did not expect %s to be present", k)
			}
		})
	}
}

func TestCompileLabelRegexes(t *testing.T) {
	tests := []struct {
		name        string
		labels      []string
		expectError bool
	}{
		{
			name:        "valid regexes",
			labels:      []string{"^tag1$", "^env_"},
			expectError: false,
		},
		{
			name:        "empty labels",
			labels:      []string{},
			expectError: false,
		},
		{
			name:        "invalid regex in config",
			labels:      []string{"[invalid"},
			expectError: true,
		},
		{
			name:        "invalid regex among valid ones",
			labels:      []string{"^valid$", "[also-invalid", "^another-valid$"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Labels: tt.labels}
			regexes, err := compileLabelRegexes(cfg)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, regexes)
			} else {
				assert.NoError(t, err)
				assert.Len(t, regexes, len(tt.labels))
			}
		})
	}
}
