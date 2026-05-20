// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vultr

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"

	vultrmeta "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/vultr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

// ---- test stub + hook ----

type fakeProvider struct {
	md  *vultrmeta.Metadata
	err error
}

func (f *fakeProvider) Metadata(_ context.Context) (*vultrmeta.Metadata, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.md, nil
}

func withFakeProvider(t *testing.T, p vultrmeta.Provider) {
	t.Helper()
	orig := newVultrProvider
	newVultrProvider = func() vultrmeta.Provider { return p }
	t.Cleanup(func() { newVultrProvider = orig })
}

// ---- tests ----

func TestNewDetector(t *testing.T) {
	withFakeProvider(t, &fakeProvider{
		md: &vultrmeta.Metadata{
			Hostname:     "vultr-guest",
			InstanceID:   "i-abc",
			InstanceV2ID: "uuid-123",
			Region:       vultrmeta.Region{RegionCode: "EWR"},
		},
	})

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	assert.NotNil(t, d)
}

func TestVultrDetector_Detect_OK(t *testing.T) {
	const (
		hostName = "vultr-guest"
		v2ID     = "36e9cf60-5d93-4e31-8ebf-613b3d2874fb"
		region   = "EWR"
	)

	withFakeProvider(t, &fakeProvider{
		md: &vultrmeta.Metadata{
			Hostname:     hostName,
			InstanceID:   "legacy-id-ignored-here",
			InstanceV2ID: v2ID,
			Region:       vultrmeta.Region{RegionCode: region},
		},
	})

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.CloudPlatform.Enabled = true
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")

	got := res.Attributes().AsRaw()
	want := map[string]any{
		"cloud.provider": TypeStr,
		"cloud.platform": TypeStr + ".cloud_compute",
		"cloud.region":   strings.ToLower(region),
		"host.id":        v2ID,
		"host.name":      hostName,
	}
	assert.Equal(t, want, got)
}

func TestVultrDetector_NotOnVultr(t *testing.T) {
	withFakeProvider(t, &fakeProvider{err: errors.New("no metadata")})

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
	assert.Empty(t, schemaURL)
}

func TestVultrDetector_FailOnMissingMetadata(t *testing.T) {
	withFakeProvider(t, &fakeProvider{err: errors.New("no metadata")})

	cfg := CreateDefaultConfig()
	cfg.FailOnMissingMetadata = true

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.Error(t, err)
	assert.True(t, internal.IsEmptyResource(res))
	assert.Empty(t, schemaURL)
}
