// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpc

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/vpc/internal/metadata"
)

const (
	hostID        = "0717_1e09281b-f177-46fb-b1f1-bc152b2e391a"
	crn           = "crn:v1:bluemix:public:is:us-south-1:a/123456789012::instance:0717_1e09281b-f177-46fb-b1f1-bc152b2e391a"
	accountID     = "123456789012"
	hostName      = "my-instance"
	hostType      = "bx2-2x8"
	zone          = "us-south-1"
	region        = "us-south"
	imageID       = "r006-ed3f775f-ad7e-4e37-ae62-7199b4988b00"
	imageName     = "ibm-ubuntu-22-04-4-minimal-amd64-3"
	cloudProvider = "ibm_cloud"
	cloudPlatform = "ibm_cloud.vpc"
)

// ---- test stub + hook ----

// fakeDetector stands in for the upstream SDK detector. Its metadata endpoint is not injectable,
// so the adapter is exercised against canned SDK results instead of a fake metadata server.
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
	newResourceDetector = func(string) sdkresource.Detector {
		return fakeDetector{res: res, err: err}
	}
	t.Cleanup(func() { newResourceDetector = orig })
}

// fullResource mirrors what the SDK detector reports for a healthy instance. It already derives
// the region from the zone and the account ID from the CRN.
func fullResource() *sdkresource.Resource {
	return sdkresource.NewSchemaless(
		attribute.String("cloud.provider", cloudProvider),
		attribute.String("cloud.platform", cloudPlatform),
		attribute.String("cloud.region", region),
		attribute.String("cloud.availability_zone", zone),
		attribute.String("cloud.account.id", accountID),
		attribute.String("cloud.resource_id", crn),
		attribute.String("host.id", hostID),
		attribute.String("host.image.id", imageID),
		attribute.String("host.image.name", imageName),
		attribute.String("host.name", hostName),
		attribute.String("host.type", hostType),
	)
}

func fullAttributes() map[string]any {
	return map[string]any{
		"cloud.provider":          cloudProvider,
		"cloud.platform":          cloudPlatform,
		"cloud.region":            region,
		"cloud.availability_zone": zone,
		"cloud.account.id":        accountID,
		"cloud.resource_id":       crn,
		"host.id":                 hostID,
		"host.image.id":           imageID,
		"host.image.name":         imageName,
		"host.name":               hostName,
		"host.type":               hostType,
	}
}

// ---- tests ----

func TestNewDetector(t *testing.T) {
	cfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), cfg, false)
	require.NoError(t, err)
	require.NotNil(t, d)
}

func TestNewDetectorHTTPS(t *testing.T) {
	cfg := CreateDefaultConfig()
	cfg.Protocol = "https"
	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), cfg, false)
	require.NoError(t, err)
	require.NotNil(t, d)
}

func TestNewDetectorInvalidProtocol(t *testing.T) {
	cfg := CreateDefaultConfig()
	cfg.Protocol = "ftp"
	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), cfg, false)
	require.Error(t, err)
	require.Nil(t, d)
	require.Contains(t, err.Error(), `invalid protocol "ftp"`)
}

func TestDetect(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, fullAttributes(), res.Attributes().AsRaw())
}

func TestDetectWithDisabledAttributes(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.CloudRegion.Enabled = false
	cfg.ResourceAttributes.HostType.Enabled = false

	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), cfg, false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	want := fullAttributes()
	delete(want, "cloud.region")
	delete(want, "host.type")
	require.Equal(t, want, res.Attributes().AsRaw())
}

func TestDetectNotOnIBMCloudVPC(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

func TestDetectEmptyResourceWithFailOnMissingMetadata(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.ErrorContains(t, err, "ibmcloud vpc metadata unavailable")
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

// The metadata service answered but the response was unusable. Without fail_on_missing_metadata
// this stays a debug log, not an error.
func TestDetectError(t *testing.T) {
	withFakeDetector(t, nil, errors.New("connection refused"))

	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

func TestDetectErrorWithFailOnMissingMetadata(t *testing.T) {
	errConnRefused := errors.New("connection refused")
	withFakeDetector(t, nil, errConnRefused)

	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, schemaURL, err := d.Detect(t.Context())
	require.ErrorIs(t, err, errConnRefused)
	require.ErrorContains(t, err, "ibmcloud vpc metadata unavailable")
	require.True(t, internal.IsEmptyResource(res))
	require.Empty(t, schemaURL)
}

// A partial result is kept as-is: the attributes absent from the metadata response are omitted
// rather than emitted with an empty value.
func TestDetectPartialMetadata(t *testing.T) {
	partial := sdkresource.NewSchemaless(
		attribute.String("cloud.provider", cloudProvider),
		attribute.String("cloud.platform", cloudPlatform),
		attribute.String("host.id", hostID),
	)
	withFakeDetector(t, partial, fmt.Errorf("%w: host.name: not present in metadata", sdkresource.ErrPartialResource))

	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), CreateDefaultConfig(), false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"cloud.provider": cloudProvider,
		"cloud.platform": cloudPlatform,
		"host.id":        hostID,
	}, res.Attributes().AsRaw())
}

// fail_on_missing_metadata covers an unusable metadata service, not a field absent from an
// otherwise good response, so a partial result still succeeds.
func TestDetectPartialMetadataWithFailOnMissingMetadata(t *testing.T) {
	partial := sdkresource.NewSchemaless(
		attribute.String("cloud.provider", cloudProvider),
		attribute.String("host.id", hostID),
	)
	withFakeDetector(t, partial, fmt.Errorf("%w: host.name: not present in metadata", sdkresource.ErrPartialResource))

	d, err := NewDetector(processortest.NewNopSettings(metadata.Type), CreateDefaultConfig(), true)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"cloud.provider": cloudProvider,
		"host.id":        hostID,
	}, res.Attributes().AsRaw())
}
