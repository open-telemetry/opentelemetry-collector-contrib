// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lambda

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
)

// Tests Lambda resource detector running in Lambda environment
func TestLambda(t *testing.T) {
	ctx := context.Background()

	const functionName = "TestFunctionName"
	t.Setenv(awsLambdaFunctionNameEnvVar, functionName)

	// Call Lambda Resource detector to detect resources
	lambdaDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	res, _, err := lambdaDetector.Detect(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, map[string]any{
		string(conventions.CloudProviderKey): conventions.CloudProviderAWS.Value.AsString(),
		string(conventions.CloudPlatformKey): conventions.CloudPlatformAWSLambda.Value.AsString(),
		string(conventions.FaaSNameKey):      functionName,
	}, res.Attributes().AsRaw(), "Resource object returned is incorrect")
}

// Tests Lambda resource detector not running in Lambda environment
func TestNotLambda(t *testing.T) {
	ctx := context.Background()
	lambdaDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	res, _, err := lambdaDetector.Detect(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, 0, res.Attributes().Len(), "Resource object should be empty")
}
