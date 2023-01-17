package lambda

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
)

func TestNewDetector(t *testing.T) {
	detector, err := NewDetector(processortest.NewNopCreateSettings(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, detector)
}

// Tests Lambda resource detector running in Lambda environment
func TestEKS(t *testing.T) {
	ctx := context.Background()

	// TODO
	// t.Setenv("AWS_LAMBDA_FUNCTION_NAME", "test-function-name")

	// Call Lambda Resource detector to detect resources
	lambdaDetector := &Detector{logger: zap.NewNop()}
	res, _, err := lambdaDetector.Detect(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, map[string]interface{}{
		"cloud.provider": "aws",
		"cloud.platform": "aws_lambda",
	}, res.Attributes().AsRaw(), "Resource object returned is incorrect")
}

// Tests Lambda resource detector not running in Lambda environment
func TestNotLambda(t *testing.T) {
	ctx := context.Background()
	lambdaDetector := &Detector{logger: zap.NewNop()}
	res, _, err := lambdaDetector.Detect(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, 0, res.Attributes().Len(), "Resource object should be empty")
}
