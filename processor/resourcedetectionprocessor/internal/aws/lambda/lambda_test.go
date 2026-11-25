// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lambda

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/lambda/internal/metadata"
)

// Tests Lambda resource detector running in Lambda environment
func TestLambda(t *testing.T) {
	ctx := t.Context()

	const functionName = "TestFunctionName"
	t.Setenv("AWS_LAMBDA_FUNCTION_NAME", functionName)

	// Call Lambda Resource detector to detect resources
	lambdaDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	res, _, err := lambdaDetector.Detect(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, map[string]any{
		"cloud.provider": "aws",
		"cloud.platform": "aws_lambda",
		"faas.name":      functionName,
	}, res.Attributes().AsRaw(), "Resource object returned is incorrect")
}

// Tests Lambda resource detector not running in Lambda environment
func TestNotLambda(t *testing.T) {
	ctx := t.Context()
	lambdaDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	res, _, err := lambdaDetector.Detect(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, 0, res.Attributes().Len(), "Resource object should be empty")
}

// Tests Lambda resource detector with every supported env var set and all attributes enabled
func TestLambdaAllAttributes(t *testing.T) {
	t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "TestFunctionName")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_LAMBDA_FUNCTION_VERSION", "$LATEST")
	t.Setenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE", "128")
	t.Setenv("AWS_LAMBDA_LOG_GROUP_NAME", "/aws/lambda/TestFunctionName")
	t.Setenv("AWS_LAMBDA_LOG_STREAM_NAME", "2026/09/27/[$LATEST]abcdef")

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.AwsLogGroupNames.Enabled = true
	cfg.ResourceAttributes.AwsLogStreamNames.Enabled = true
	cfg.ResourceAttributes.CloudPlatform.Enabled = true
	cfg.ResourceAttributes.CloudProvider.Enabled = true
	cfg.ResourceAttributes.CloudRegion.Enabled = true
	cfg.ResourceAttributes.FaasInstance.Enabled = true
	cfg.ResourceAttributes.FaasMaxMemory.Enabled = true
	cfg.ResourceAttributes.FaasName.Enabled = true
	cfg.ResourceAttributes.FaasVersion.Enabled = true

	lambdaDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)
	res, schemaURL, err := lambdaDetector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")

	assert.Equal(t, map[string]any{
		"cloud.provider":       "aws",
		"cloud.platform":       "aws_lambda",
		"cloud.region":         "us-east-1",
		"faas.name":            "TestFunctionName",
		"faas.version":         "$LATEST",
		"faas.instance":        "2026/09/27/[$LATEST]abcdef",
		"faas.max_memory":      "128",
		"aws.log.group.names":  []any{"/aws/lambda/TestFunctionName"},
		"aws.log.stream.names": []any{"2026/09/27/[$LATEST]abcdef"},
	}, res.Attributes().AsRaw())
}

// Tests Lambda resource detector outside Lambda with fail_on_missing_metadata enabled
func TestNotLambdaFailOnMissingMetadata(t *testing.T) {
	lambdaDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), true)
	require.NoError(t, err)
	res, schemaURL, err := lambdaDetector.Detect(t.Context())
	require.EqualError(t, err, "lambda metadata unavailable: AWS_LAMBDA_FUNCTION_NAME env var not set")
	assert.Empty(t, schemaURL)
	assert.Equal(t, 0, res.Attributes().Len())
}

// Tests that enabling the faasMaxMemoryBytes gate emits faas.max_memory as an int in bytes
func TestLambdaMaxMemoryGateEnabled(t *testing.T) {
	gate := metadata.ProcessorResourcedetectionLambdaFaasMaxMemoryBytesFeatureGate
	original := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), true))
	t.Cleanup(func() { require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), original)) })

	t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "TestFunctionName")
	t.Setenv("AWS_LAMBDA_FUNCTION_MEMORY_SIZE", "128")

	lambdaDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig(), false)
	require.NoError(t, err)
	res, _, err := lambdaDetector.Detect(t.Context())
	require.NoError(t, err)

	v, ok := res.Attributes().Get("faas.max_memory")
	require.True(t, ok)
	assert.Equal(t, int64(128<<20), v.Int())
}
