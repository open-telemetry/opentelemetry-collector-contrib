// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azure

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/internal/metadata"
)

const testSchemaURL = "https://opentelemetry.io/schemas/1.43.0"

// fakeDetector stands in for the upstream SDK detector, whose metadata endpoint is not injectable.
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
	newResourceDetector = func(func(string) bool) sdkresource.Detector {
		return fakeDetector{res: res, err: err}
	}
	t.Cleanup(func() { newResourceDetector = orig })
}

// sdkAttributes mirrors what the SDK detector reports for a healthy VM, including the
// attributes this detector does not declare.
func sdkAttributes() []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("cloud.provider", "azure"),
		attribute.String("cloud.platform", "azure.vm"),
		attribute.String("host.id", "vmID"),
		attribute.String("cloud.region", "location"),
		attribute.String("cloud.resource_id", "/subscriptions/subscriptionID/vm"),
		attribute.String("host.name", "computerName"),
		attribute.String("host.type", "vmSize"),
		attribute.String("os.type", "Linux"),
		attribute.String("os.version", "1.2.3"),
		attribute.String("cloud.account.id", "subscriptionID"),
		attribute.String("cloud.availability_zone", "zone"),
		attribute.String("azure.vm.name", "name"),
		attribute.String("azure.vm.size", "vmSize"),
		attribute.String("azure.vm.scaleset.name", "myScaleset"),
		attribute.String("azure.resource_group.name", "resourceGroup"),
	}
}

func sdkResource() *sdkresource.Resource {
	return sdkresource.NewWithAttributes(testSchemaURL, sdkAttributes()...)
}

func newTestDetector(t *testing.T, cfg Config, failOnMissingMetadata bool) internal.Detector {
	t.Helper()

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, failOnMissingMetadata)
	require.NoError(t, err)
	return d
}

func TestNewDetector(t *testing.T) {
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	assert.NotNil(t, d)
}

func TestNewDetectorInvalidTagRegex(t *testing.T) {
	cfg := CreateDefaultConfig()
	cfg.Tags = []string{"("}
	_, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.Error(t, err)
}

// The default output is exactly the declared and enabled attributes: cloud.availability_zone
// is disabled by default and the SDK's undeclared attributes must not leak through.
func TestDetectDefaultConfig(t *testing.T) {
	withFakeDetector(t, sdkResource(), nil)

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, testSchemaURL, schemaURL)
	assert.Equal(t, map[string]any{
		"cloud.provider":           "azure",
		"cloud.platform":           "azure.vm",
		"host.name":                "computerName",
		"cloud.region":             "location",
		"host.id":                  "vmID",
		"cloud.account.id":         "subscriptionID",
		"azure.vm.name":            "name",
		"azure.vm.size":            "vmSize",
		"azure.resourcegroup.name": "resourceGroup",
		"azure.vm.scaleset.name":   "myScaleset",
	}, res.Attributes().AsRaw())
}

func TestDetectConfiguredAttributes(t *testing.T) {
	withFakeDetector(t, sdkResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.CloudAvailabilityZone.Enabled = true
	cfg.ResourceAttributes.HostName.Enabled = false

	res, _, err := newTestDetector(t, cfg, false).Detect(t.Context())
	require.NoError(t, err)

	zone, ok := res.Attributes().Get("cloud.availability_zone")
	require.True(t, ok)
	assert.Equal(t, "zone", zone.Str())
	_, ok = res.Attributes().Get("host.name")
	assert.False(t, ok)
}

func TestDetectResourceGroupNameFeatureGate(t *testing.T) {
	for _, tt := range []struct {
		name        string
		gateEnabled bool
		wantKey     string
		notWantKey  string
	}{
		{
			name:        "gate disabled emits the legacy attribute",
			gateEnabled: false,
			wantKey:     "azure.resourcegroup.name",
			notWantKey:  "azure.resource_group.name",
		},
		{
			name:        "gate enabled emits the semconv aligned attribute",
			gateEnabled: true,
			wantKey:     "azure.resource_group.name",
			notWantKey:  "azure.resourcegroup.name",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gateID := metadata.ProcessorAzuredetectorRenameResourceGroupAttributeFeatureGate.ID()
			require.NoError(t, featuregate.GlobalRegistry().Set(gateID, tt.gateEnabled))
			t.Cleanup(func() {
				require.NoError(t, featuregate.GlobalRegistry().Set(gateID, false))
			})

			withFakeDetector(t, sdkResource(), nil)

			res, _, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
			require.NoError(t, err)

			got, ok := res.Attributes().Get(tt.wantKey)
			require.True(t, ok, "expected %q to be present", tt.wantKey)
			assert.Equal(t, "resourceGroup", got.Str())

			_, ok = res.Attributes().Get(tt.notWantKey)
			assert.False(t, ok, "expected %q to be absent", tt.notWantKey)
		})
	}
}

// The configured regexes reach the SDK detector as a single OR matcher, and the tags it
// reports survive even though they are not declared in metadata.yaml.
func TestDetectTags(t *testing.T) {
	orig := newResourceDetector
	t.Cleanup(func() { newResourceDetector = orig })
	newResourceDetector = func(tagKeyFilter func(string) bool) sdkresource.Detector {
		attrs := sdkAttributes()
		for _, tag := range []string{"tag1key", "tag2key", "other"} {
			if tagKeyFilter(tag) {
				attrs = append(attrs, attribute.String(tagPrefix+tag, tag+"-value"))
			}
		}
		return fakeDetector{res: sdkresource.NewWithAttributes(testSchemaURL, attrs...)}
	}

	cfg := CreateDefaultConfig()
	cfg.Tags = []string{"^tag1key$", "^tag2"}

	res, _, err := newTestDetector(t, cfg, false).Detect(t.Context())
	require.NoError(t, err)

	raw := res.Attributes().AsRaw()
	assert.Equal(t, "tag1key-value", raw["azure.tag.tag1key"])
	assert.Equal(t, "tag2key-value", raw["azure.tag.tag2key"])
	assert.NotContains(t, raw, "azure.tag.other")
}

func TestDetectNoTagsConfigured(t *testing.T) {
	orig := newResourceDetector
	t.Cleanup(func() { newResourceDetector = orig })
	var filter func(string) bool
	newResourceDetector = func(tagKeyFilter func(string) bool) sdkresource.Detector {
		filter = tagKeyFilter
		return fakeDetector{res: sdkResource()}
	}

	newTestDetector(t, CreateDefaultConfig(), false)
	require.NotNil(t, filter)
	assert.False(t, filter("anything"))
}

// Attributes the SDK did not report stay absent rather than being emitted empty.
func TestDetectAbsentAttributesOmitted(t *testing.T) {
	withFakeDetector(t, sdkresource.NewWithAttributes(testSchemaURL,
		attribute.String("cloud.provider", "azure"),
		attribute.String("host.id", "vmID"),
	), nil)

	res, _, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"cloud.provider": "azure",
		"host.id":        "vmID",
	}, res.Attributes().AsRaw())
}

// The ResourceBuilder is shared across calls: nothing may carry over from an earlier Detect.
func TestDetectRepeatedDoesNotRetainAttributes(t *testing.T) {
	withFakeDetector(t, sdkResource(), nil)

	d := newTestDetector(t, CreateDefaultConfig(), false)
	_, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	d.(*Detector).detector = fakeDetector{res: sdkresource.NewWithAttributes(testSchemaURL,
		attribute.String("host.id", "otherVMID"),
	)}
	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"host.id": "otherVMID"}, res.Attributes().AsRaw())
}

func TestDetectNotOnAzure(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
	assert.Empty(t, schemaURL)
}

func TestDetectNotOnAzureFailOnMissingMetadata(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), true).Detect(t.Context())
	require.EqualError(t, err, "azure metadata unavailable")
	assert.True(t, internal.IsEmptyResource(res))
	assert.Empty(t, schemaURL)
}

func TestDetectError(t *testing.T) {
	withFakeDetector(t, nil, errors.New("mock error"))

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
	assert.Empty(t, schemaURL)
}

func TestDetectErrorFailOnMissingMetadata(t *testing.T) {
	errMock := errors.New("mock error")
	withFakeDetector(t, nil, errMock)

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), true).Detect(t.Context())
	require.ErrorIs(t, err, errMock)
	assert.True(t, internal.IsEmptyResource(res))
	assert.Empty(t, schemaURL)
}

func TestDetectPartialResult(t *testing.T) {
	partial := sdkresource.NewWithAttributes(testSchemaURL,
		attribute.String("cloud.provider", "azure"),
		attribute.String("host.name", "computerName"),
	)
	partialErr := fmt.Errorf("%w: missing fields", sdkresource.ErrPartialResource)

	for _, failOnMissingMetadata := range []bool{false, true} {
		withFakeDetector(t, partial, partialErr)

		res, _, err := newTestDetector(t, CreateDefaultConfig(), failOnMissingMetadata).Detect(t.Context())
		require.NoError(t, err)
		assert.Equal(t, map[string]any{
			"cloud.provider": "azure",
			"host.name":      "computerName",
		}, res.Attributes().AsRaw())
	}
}
