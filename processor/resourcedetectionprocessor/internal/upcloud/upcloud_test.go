// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upcloud

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"

	upcloudmeta "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/upcloud"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

// ---- test stub + hook ----

type fakeProvider struct {
	md  *upcloudmeta.Metadata
	err error
}

func (f *fakeProvider) Metadata(_ context.Context) (*upcloudmeta.Metadata, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.md, nil
}

func withFakeProvider(t *testing.T, p upcloudmeta.Provider) {
	t.Helper()
	orig := newUpcloudProvider
	newUpcloudProvider = func() upcloudmeta.Provider { return p }
	t.Cleanup(func() { newUpcloudProvider = orig })
}

// ---- tests ----

func TestNewDetector(t *testing.T) {
	withFakeProvider(t, &fakeProvider{
		md: &upcloudmeta.Metadata{
			CloudName:  "upcloud",
			Hostname:   "ubuntu-1cpu-1gb-es-mad1",
			InstanceID: "00133099-f1fd-4ed2-b1c7-d027eb43a8f5",
			Region:     "es-mad1",
		},
	})

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	assert.NotNil(t, d)
}

func TestUpcloudDetector_Detect_OK(t *testing.T) {
	const (
		cloud      = "upcloud"
		hostName   = "ubuntu-1cpu-1gb-es-mad1"
		instanceID = "00133099-f1fd-4ed2-b1c7-d027eb43a8f5"
		region     = "es-mad1"
	)

	withFakeProvider(t, &fakeProvider{
		md: &upcloudmeta.Metadata{
			CloudName:  cloud,
			Hostname:   hostName,
			InstanceID: instanceID,
			Region:     region,
		},
	})

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, conventions.SchemaURL, schemaURL)

	got := res.Attributes().AsRaw()
	want := map[string]any{
		string(conventions.CloudProviderKey): cloud,
		string(conventions.CloudRegionKey):   region,
		string(conventions.HostIDKey):        instanceID,
		string(conventions.HostNameKey):      hostName,
	}
	assert.Equal(t, want, got)
}

func TestUpcloudDetector_NotOnUpcloud(t *testing.T) {
	withFakeProvider(t, &fakeProvider{err: errors.New("no metadata")})

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
	assert.Empty(t, schemaURL)
}

func TestUpcloudDetector_FailOnMissingMetadata(t *testing.T) {
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
