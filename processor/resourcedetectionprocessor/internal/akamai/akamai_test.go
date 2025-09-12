// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package akamai

import (
	"context"
	"errors"
	"strconv"
	"testing"

	linodemeta "github.com/linode/go-metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

// ---- test fakes & helpers ----

type fakeAkamaiClient struct {
	inst *linodemeta.InstanceData
	err  error
}

func (f *fakeAkamaiClient) GetInstance(_ context.Context) (*linodemeta.InstanceData, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.inst, nil
}

func withFakeClient(t *testing.T, cli akamaiAPI) {
	t.Helper()
	orig := newAkamaiClient
	newAkamaiClient = func(_ context.Context) (akamaiAPI, error) { return cli, nil }
	t.Cleanup(func() { newAkamaiClient = orig })
}

// ---- tests ----

func TestNewDetector(t *testing.T) {
	withFakeClient(t, &fakeAkamaiClient{
		inst: &linodemeta.InstanceData{
			ID:     1,
			Label:  "dummy",
			Region: "us-east",
			Type:   "g6-standard-2",
			Image:  linodemeta.InstanceImageData{ID: "linode/ubuntu22.04"},
		},
	})

	det, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	require.NotNil(t, det)
}

func TestAkamaiDetector_Detect_OK(t *testing.T) {
	const (
		cloudProvider = "akamai_cloud"
		cloudPlatform = "akamai_cloud_platform"
		acct          = "acc-eeee-uuuu-iiiii-dddd"
		id            = 4242
		label         = "linode-4242"
		instanceType  = "g6-standard-4"
		region        = "us-southeast"
		imageID       = "linode/ubuntu24.04"
		imageLabel    = "Ubuntu 24.04 LTS"
	)

	withFakeClient(t, &fakeAkamaiClient{
		inst: &linodemeta.InstanceData{
			ID:           id,
			Label:        label,
			Region:       region,
			Type:         instanceType,
			Image:        linodemeta.InstanceImageData{ID: imageID, Label: imageLabel},
			AccountEUUID: acct,
		},
	})

	det, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)

	res, schemaURL, err := det.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, conventions.SchemaURL, schemaURL)

	got := res.Attributes().AsRaw()
	want := map[string]any{
		string(conventions.CloudPlatformKey):  cloudPlatform,
		string(conventions.CloudProviderKey):  cloudProvider,
		string(conventions.CloudRegionKey):    region,
		string(conventions.CloudAccountIDKey): acct,
		string(conventions.HostIDKey):         strconv.Itoa(id),
		string(conventions.HostNameKey):       label,
		string(conventions.HostTypeKey):       instanceType,
		string(conventions.HostImageIDKey):    imageID,
		string(conventions.HostImageNameKey):  imageLabel,
	}
	assert.Equal(t, want, got)
}

func TestAkamaiDetector_NotOnAkamai(t *testing.T) {
	// Pretend we are not on Akamai / metadata unreachable.
	withFakeClient(t, &fakeAkamaiClient{
		err: errors.New("no metadata here"),
	})

	det, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)

	res, schemaURL, err := det.Detect(t.Context())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))
	assert.Empty(t, schemaURL)
}
