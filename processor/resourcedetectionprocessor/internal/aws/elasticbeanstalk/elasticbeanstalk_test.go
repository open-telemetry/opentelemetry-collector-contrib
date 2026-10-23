// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticbeanstalk

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/elasticbeanstalk/internal/metadata"
)

const (
	deploymentID    = "23"
	environmentName = "BETA"
	versionLabel    = "env-version-1234"
)

// fakeDetector stands in for the upstream SDK detector, whose configuration file
// path is not injectable.
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

func setGate(t *testing.T, gate *featuregate.Gate, enabled bool) {
	t.Helper()

	original := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), original))
	})
}

func setEmitV1(t *testing.T, enabled bool) {
	t.Helper()
	setGate(t, metadata.ProcessorResourcedetectionElasticbeanstalkEmitV1DeploymentConventionsFeatureGate, enabled)
}

func setDontEmitV0(t *testing.T, enabled bool) {
	t.Helper()
	setGate(t, metadata.ProcessorResourcedetectionElasticbeanstalkDontEmitV0DeploymentConventionsFeatureGate, enabled)
}

func fullResource() *sdkresource.Resource {
	return sdkresource.NewSchemaless(
		attribute.String("cloud.provider", "aws"),
		attribute.String("cloud.platform", "aws_elastic_beanstalk"),
		attribute.String("deployment.id", deploymentID),
		attribute.String("deployment.environment.name", environmentName),
		attribute.String("service.version", versionLabel),
	)
}

func newTestDetector(t *testing.T, failOnMissingMetadata bool) internal.Detector {
	t.Helper()

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), failOnMissingMetadata)
	require.NoError(t, err)
	return d
}

// Default gates must report what the detector reported before delegating to the SDK.
func TestDetect_DefaultGates(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	d := newTestDetector(t, false)
	res, schemaURL, err := d.Detect(t.Context())
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"cloud.provider":         "aws",
		"cloud.platform":         "aws_elastic_beanstalk",
		"deployment.environment": environmentName,
		"service.instance.id":    deploymentID,
		"service.version":        versionLabel,
	}, res.Attributes().AsRaw())
	assert.Equal(t, "https://opentelemetry.io/schemas/1.40.0", schemaURL)
}

func TestDetect_EmitV1Only(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)
	setEmitV1(t, true)

	d := newTestDetector(t, false)
	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"cloud.provider":              "aws",
		"cloud.platform":              "aws_elastic_beanstalk",
		"deployment.environment":      environmentName,
		"deployment.environment.name": environmentName,
		"deployment.id":               deploymentID,
		"service.instance.id":         deploymentID,
		"service.version":             versionLabel,
	}, res.Attributes().AsRaw())
}

func TestDetect_BothGates(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)
	setEmitV1(t, true)
	setDontEmitV0(t, true)

	d := newTestDetector(t, false)
	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"cloud.provider":              "aws",
		"cloud.platform":              "aws_elastic_beanstalk",
		"deployment.environment.name": environmentName,
		"deployment.id":               deploymentID,
		"service.version":             versionLabel,
	}, res.Attributes().AsRaw())
}

func TestNewDetector_DontEmitV0WithoutEmitV1(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)
	setDontEmitV0(t, true)

	_, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.ErrorContains(t, err, "DontEmitV0DeploymentConventions requires")
}

func TestDetect_ResourceAttributesDisabled(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.DeploymentEnvironment.Enabled = false
	cfg.ResourceAttributes.ServiceVersion.Enabled = false

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)

	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"cloud.provider":      "aws",
		"cloud.platform":      "aws_elastic_beanstalk",
		"service.instance.id": deploymentID,
	}, res.Attributes().AsRaw())
}

func TestDetect_NotOnElasticBeanstalk(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d := newTestDetector(t, false)
	res, schemaURL, err := d.Detect(t.Context())

	require.NoError(t, err)
	assert.Equal(t, 0, res.Attributes().Len())
	assert.Empty(t, schemaURL)
}

func TestDetect_NotOnElasticBeanstalkFailOnMissingMetadata(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	d := newTestDetector(t, true)
	res, _, err := d.Detect(t.Context())

	require.ErrorContains(t, err, "elastic_beanstalk metadata unavailable")
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestDetect_Error(t *testing.T) {
	detectErr := errors.New("elasticbeanstalk: invalid character 's'")
	withFakeDetector(t, sdkresource.Empty(), detectErr)

	d := newTestDetector(t, false)
	res, _, err := d.Detect(t.Context())

	require.ErrorIs(t, err, detectErr)
	assert.Equal(t, 0, res.Attributes().Len())
}

func TestDetect_PartialResource(t *testing.T) {
	partial := sdkresource.NewSchemaless(
		attribute.String("cloud.provider", "aws"),
		attribute.String("cloud.platform", "aws_elastic_beanstalk"),
	)
	withFakeDetector(t, partial, sdkresource.ErrPartialResource)

	d := newTestDetector(t, false)
	res, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"cloud.provider": "aws",
		"cloud.platform": "aws_elastic_beanstalk",
	}, res.Attributes().AsRaw())
}
