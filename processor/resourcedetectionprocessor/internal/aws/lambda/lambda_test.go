// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lambda

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

func TestNewDetector(t *testing.T) {
	dcfg := CreateDefaultConfig()
	detector, err := NewDetector(processortest.NewNopCreateSettings(), dcfg)
	assert.NoError(t, err)
	assert.NotNil(t, detector)
}

// Tests Lambda resource detector running in Lambda environment
func TestLambda(t *testing.T) {
	ctx := context.Background()

	const functionName = "TestFunctionName"
	t.Setenv(awsLambdaFunctionNameEnvVar, functionName)

	// Call Lambda Resource detector to detect resources
	resourceAttributes := CreateDefaultConfig().ResourceAttributes
	lambdaDetector := &detector{logger: zap.NewNop(), resourceAttributes: resourceAttributes}
	res, _, err := lambdaDetector.Detect(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, map[string]interface{}{
		conventions.AttributeCloudProvider: conventions.AttributeCloudProviderAWS,
		conventions.AttributeCloudPlatform: conventions.AttributeCloudPlatformAWSLambda,
		conventions.AttributeFaaSName:      functionName,
	}, res.Attributes().AsRaw(), "Resource object returned is incorrect")
}

// Tests Lambda resource detector not running in Lambda environment
func TestNotLambda(t *testing.T) {
	ctx := context.Background()
	resourceAttributes := CreateDefaultConfig().ResourceAttributes
	lambdaDetector := &detector{logger: zap.NewNop(), resourceAttributes: resourceAttributes}
	res, _, err := lambdaDetector.Detect(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, 0, res.Attributes().Len(), "Resource object should be empty")
}
