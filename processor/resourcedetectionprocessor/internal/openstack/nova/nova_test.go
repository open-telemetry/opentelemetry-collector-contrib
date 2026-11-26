// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nova

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
)

const testSchemaURL = "https://opentelemetry.io/schemas/1.43.0"

// fakeDetector stands in for the upstream SDK detector, whose metadata endpoint is
// not injectable. Like the SDK detector, it emits the entries of meta whose key
// satisfies filter, and none when filter is nil.
type fakeDetector struct {
	attrs  []attribute.KeyValue
	meta   map[string]string
	filter func(string) bool
	err    error
}

func (f fakeDetector) Detect(context.Context) (*sdkresource.Resource, error) {
	attrs := f.attrs
	for k, v := range f.meta {
		if f.filter != nil && f.filter(k) {
			attrs = append(attrs, attribute.String(LabelPrefix+k, v))
		}
	}
	return sdkresource.NewWithAttributes(testSchemaURL, attrs...), f.err
}

func withFakeDetector(t *testing.T, f fakeDetector) {
	t.Helper()

	orig := newResourceDetector
	newResourceDetector = func(metaKeyFilter func(string) bool) sdkresource.Detector {
		f.filter = metaKeyFilter
		return f
	}
	t.Cleanup(func() { newResourceDetector = orig })
}

func fullAttributes() []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("cloud.provider", "openstack"),
		attribute.String("cloud.platform", "openstack_nova"),
		attribute.String("cloud.account.id", "proj-xyz"),
		attribute.String("cloud.availability_zone", "zone-a"),
		attribute.String("host.id", "vm-1234"),
		attribute.String("host.name", "example-nova-host"),
		attribute.String("host.type", "dummy.host.type"),
	}
}

func fullWant() map[string]any {
	return map[string]any{
		"cloud.provider":          "openstack",
		"cloud.platform":          "openstack_nova",
		"cloud.account.id":        "proj-xyz",
		"cloud.availability_zone": "zone-a",
		"host.id":                 "vm-1234",
		"host.name":               "example-nova-host",
		"host.type":               "dummy.host.type",
	}
}

func TestNewDetector(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		shouldError bool
	}{
		{
			name:        "Success Case Empty Config",
			cfg:         Config{},
			shouldError: false,
		},
		{
			name: "Success Case Valid Config",
			cfg: Config{
				Labels: []string{"label1"},
			},
			shouldError: false,
		},
		{
			name: "Error Case Invalid Regex",
			cfg: Config{
				Labels: []string{"*"},
			},
			shouldError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), tt.cfg, false)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, detector)
			} else {
				assert.NotNil(t, detector)
				assert.NoError(t, err)
			}
		})
	}
}

func TestDetect_OK(t *testing.T) {
	withFakeDetector(t, fakeDetector{attrs: fullAttributes()})

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, testSchemaURL, schemaURL)
	assert.Equal(t, fullWant(), res.Attributes().AsRaw())
}

func TestDetect_Labels(t *testing.T) {
	meta := map[string]string{"tag1": "val1", "tag2": "val2", "other": "nope"}

	tests := []struct {
		name    string
		labels  []string
		wantTag bool
	}{
		{name: "matching any regex is enough", labels: []string{"^tag1$", "^tag2$"}, wantTag: true},
		{name: "no labels emit no meta keys"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withFakeDetector(t, fakeDetector{attrs: fullAttributes(), meta: meta})

			cfg := CreateDefaultConfig()
			cfg.Labels = tt.labels
			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
			require.NoError(t, err)

			res, _, err := d.Detect(t.Context())
			require.NoError(t, err)

			want := fullWant()
			if tt.wantTag {
				want["openstack.nova.meta.tag1"] = "val1"
				want["openstack.nova.meta.tag2"] = "val2"
			}
			assert.Equal(t, want, res.Attributes().AsRaw())
		})
	}
}

// Disabling declared attributes must not affect the dynamic meta keys.
func TestDetect_DisabledAttributeRemoved(t *testing.T) {
	withFakeDetector(t, fakeDetector{attrs: fullAttributes(), meta: map[string]string{"tag1": "val1"}})

	cfg := CreateDefaultConfig()
	cfg.Labels = []string{"^tag1$"}
	cfg.ResourceAttributes.HostName.Enabled = false
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := fullWant()
	delete(want, "host.name")
	want["openstack.nova.meta.tag1"] = "val1"
	assert.Equal(t, want, res.Attributes().AsRaw())
}

func TestDetect_EmptyResult(t *testing.T) {
	withFakeDetector(t, fakeDetector{})

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 0, res.Attributes().Len())
	assert.Empty(t, schemaURL)
}

func TestDetect_EmptyResult_FailOnMissingMetadata(t *testing.T) {
	withFakeDetector(t, fakeDetector{})

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.EqualError(t, err, "nova metadata unavailable")
	assert.Equal(t, 0, res.Attributes().Len())
	assert.Empty(t, schemaURL)
}

func TestDetect_Error(t *testing.T) {
	withFakeDetector(t, fakeDetector{err: errors.New("no metadata")})

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 0, res.Attributes().Len())
	assert.Empty(t, schemaURL)
}

func TestDetect_Error_FailOnMissingMetadata(t *testing.T) {
	errNoMetadata := errors.New("no metadata")
	withFakeDetector(t, fakeDetector{err: errNoMetadata})

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.ErrorIs(t, err, errNoMetadata)
	assert.Equal(t, 0, res.Attributes().Len())
	assert.Empty(t, schemaURL)
}

// The deprecated per-detector flag alone must still trigger fail-on-missing.
func TestDetect_DeprecatedFailOnMissingMetadata(t *testing.T) {
	withFakeDetector(t, fakeDetector{})

	cfg := CreateDefaultConfig()
	cfg.FailOnMissingMetadata = true
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	_, _, err = d.Detect(t.Context())
	require.Error(t, err)
}

// fail_on_missing_metadata covers an unusable metadata service, not a field absent
// from an otherwise good response, so a partial result succeeds with either setting.
func TestDetect_Partial(t *testing.T) {
	for _, failOnMissingMetadata := range []bool{false, true} {
		t.Run(fmt.Sprintf("fail_on_missing_metadata=%t", failOnMissingMetadata), func(t *testing.T) {
			withFakeDetector(t, fakeDetector{
				attrs: []attribute.KeyValue{
					attribute.String("cloud.provider", "openstack"),
					attribute.String("cloud.platform", "openstack_nova"),
				},
				meta: map[string]string{"tag1": "val1"},
				err:  fmt.Errorf("%w: hostname: not present in metadata", sdkresource.ErrPartialResource),
			})

			cfg := CreateDefaultConfig()
			cfg.Labels = []string{"^tag1$"}
			d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, failOnMissingMetadata)
			require.NoError(t, err)

			res, schemaURL, err := d.Detect(t.Context())
			require.NoError(t, err)
			assert.Equal(t, testSchemaURL, schemaURL)
			assert.Equal(t, map[string]any{
				"cloud.provider":           "openstack",
				"cloud.platform":           "openstack_nova",
				"openstack.nova.meta.tag1": "val1",
			}, res.Attributes().AsRaw())
		})
	}
}
