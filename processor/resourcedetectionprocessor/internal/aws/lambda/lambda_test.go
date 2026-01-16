// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lambda

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
)

// Tests Lambda resource detector running in Lambda environment
func TestLambda(t *testing.T) {
	ctx := t.Context()

	const functionName = "TestFunctionName"
	t.Setenv(awsLambdaFunctionNameEnvVar, functionName)

	// Call Lambda Resource detector to detect resources
	lambdaDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
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
	lambdaDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	res, _, err := lambdaDetector.Detect(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, 0, res.Attributes().Len(), "Resource object should be empty")
}
