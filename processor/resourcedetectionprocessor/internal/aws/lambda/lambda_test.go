// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lambda

import (
	"os"
	"path/filepath"
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

// Tests that cloud.account.id is set when the account ID symlink exists
func TestLambdaAccountIDSymlink(t *testing.T) {
	ctx := t.Context()

	const functionName = "TestFunctionName"
	const expectedAccountID = "123456789012"
	t.Setenv(awsLambdaFunctionNameEnvVar, functionName)

	// Create a symlink at a temp path and override the constant via a helper.
	// Since accountIDSymlinkPath is a const, we create the symlink at the actual path
	// in a temp directory and override it for the test.
	tmpDir := t.TempDir()
	symlinkPath := filepath.Join(tmpDir, ".otel-account-id")
	err := os.Symlink(expectedAccountID, symlinkPath)
	require.NoError(t, err)

	// Patch the global symlink path for this test by creating the symlink at the well-known path.
	// We use the actual /tmp path since that's what the detector reads.
	// Clean up any pre-existing symlink first (ignore error if it doesn't exist).
	os.Remove(accountIDSymlinkPath)
	t.Cleanup(func() { os.Remove(accountIDSymlinkPath) })
	err = os.Symlink(expectedAccountID, accountIDSymlinkPath)
	require.NoError(t, err)

	lambdaDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	res, _, err := lambdaDetector.Detect(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)

	val, ok := res.Attributes().Get("cloud.account.id")
	assert.True(t, ok, "cloud.account.id attribute should be present")
	assert.Equal(t, expectedAccountID, val.Str())
}

// Tests that cloud.account.id is absent when the symlink does not exist
func TestLambdaAccountIDSymlinkMissing(t *testing.T) {
	ctx := t.Context()

	const functionName = "TestFunctionName"
	t.Setenv(awsLambdaFunctionNameEnvVar, functionName)

	// Ensure symlink does not exist
	os.Remove(accountIDSymlinkPath)

	lambdaDetector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	res, _, err := lambdaDetector.Detect(ctx)
	require.NoError(t, err)
	require.NotNil(t, res)

	_, ok := res.Attributes().Get("cloud.account.id")
	assert.False(t, ok, "cloud.account.id attribute should not be present when symlink is missing")
	// Verify other attributes are still set correctly
	assert.Equal(t, "aws", res.Attributes().AsRaw()["cloud.provider"])
	assert.Equal(t, "aws_lambda", res.Attributes().AsRaw()["cloud.platform"])
	assert.Equal(t, functionName, res.Attributes().AsRaw()["faas.name"])
}
