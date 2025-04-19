// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package endpoints

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetTMEV4FromEnv tests retrieving task metadata endpoint from V4 environment variable
func TestGetTMEV4FromEnv(t *testing.T) {
	// Set environment variable for V4 endpoint
	t.Setenv(TaskMetadataEndpointV4EnvVar, "http://169.254.170.2/v4")

	endpoint, err := GetTMEV4FromEnv()
	require.NoError(t, err)
	require.NotNil(t, endpoint)
	assert.Equal(t, "169.254.170.2", endpoint.Host)
	assert.Equal(t, "/v4", endpoint.Path)
	assert.Equal(t, "http", endpoint.Scheme)
}

// TestGetTMEV3FromEnv tests retrieving task metadata endpoint from V3 environment variable
func TestGetTMEV3FromEnv(t *testing.T) {
	t.Setenv(TaskMetadataEndpointV3EnvVar, "http://169.254.170.2/v3")
	t.Setenv(TaskMetadataEndpointV4EnvVar, "") // Clear V4 to isolate test

	endpoint, err := GetTMEV3FromEnv()
	require.NoError(t, err)
	require.NotNil(t, endpoint)
	assert.Equal(t, "169.254.170.2", endpoint.Host)
	assert.Equal(t, "/v3", endpoint.Path)
	assert.Equal(t, "http", endpoint.Scheme)
}

// TestGetTMEPreference tests the preference for V4 endpoint over V3
func TestGetTMEPreference(t *testing.T) {
	t.Setenv(TaskMetadataEndpointV3EnvVar, "http://169.254.170.2/v3")
	t.Setenv(TaskMetadataEndpointV4EnvVar, "http://169.254.170.2/v4")

	endpoint, err := GetTMEFromEnv()
	require.NoError(t, err)
	require.NotNil(t, endpoint)
	assert.Equal(t, "/v4", endpoint.Path, "Should prefer V4 endpoint when both are available")
}

// TestGetTMEFallback tests falling back to V3 when V4 is not available
func TestGetTMEFallback(t *testing.T) {
	t.Setenv(TaskMetadataEndpointV3EnvVar, "http://169.254.170.2/v3")
	t.Setenv(TaskMetadataEndpointV4EnvVar, "") // V4 not available

	endpoint, err := GetTMEFromEnv()
	require.NoError(t, err)
	require.NotNil(t, endpoint)
	assert.Equal(t, "/v3", endpoint.Path, "Should fall back to V3 endpoint when V4 is not available")
}

// TestNoEndpoints tests behavior when no endpoints are available
func TestNoEndpoints(t *testing.T) {
	t.Setenv(TaskMetadataEndpointV3EnvVar, "")
	t.Setenv(TaskMetadataEndpointV4EnvVar, "")

	endpoint, err := GetTMEFromEnv()
	assert.Error(t, err, "Should return error when no endpoints are available")
	assert.Nil(t, endpoint)

	// Verify the error is the expected type
	var v3Err ErrNoTaskMetadataEndpointDetected
	if assert.ErrorAs(t, err, &v3Err) {
		assert.Equal(t, 3, v3Err.MissingVersion)
	}
}

// TestInvalidEndpointURL tests handling invalid endpoint URLs
func TestInvalidEndpointURL(t *testing.T) {
	// Go's URL parser is surprisingly permissive, so we need a really malformed URL
	// for this test to work properly
	t.Setenv(TaskMetadataEndpointV4EnvVar, "http://[invalid-host")

	endpoint, err := GetTMEV4FromEnv()
	assert.Error(t, err)
	assert.Nil(t, endpoint)

	// Verify the error is the expected type
	var noEndpointErr ErrNoTaskMetadataEndpointDetected
	if assert.ErrorAs(t, err, &noEndpointErr) {
		assert.Equal(t, 4, noEndpointErr.MissingVersion)
	}
}

// TestEmptyEndpointURL tests handling empty endpoint URLs
func TestEmptyEndpointURL(t *testing.T) {
	t.Setenv(TaskMetadataEndpointV4EnvVar, "")

	endpoint, err := GetTMEV4FromEnv()
	assert.Error(t, err)
	assert.Nil(t, endpoint)

	var noEndpointErr ErrNoTaskMetadataEndpointDetected
	if assert.ErrorAs(t, err, &noEndpointErr) {
		assert.Equal(t, 4, noEndpointErr.MissingVersion)
	}
}

// TestValidateEndpoint tests the validateEndpoint helper function
func TestValidateEndpoint(t *testing.T) {
	// Valid endpoint
	endpoint, err := validateEndpoint("http://169.254.170.2/v4")
	require.NoError(t, err)
	require.NotNil(t, endpoint)
	assert.Equal(t, "169.254.170.2", endpoint.Host)

	// Empty endpoint
	endpoint, err = validateEndpoint("")
	assert.Error(t, err)
	assert.Nil(t, endpoint)
	assert.Contains(t, err.Error(), "endpoint is empty")

	// Invalid URL
	endpoint, err = validateEndpoint("http://[invalid-host")
	assert.Error(t, err)
	assert.Nil(t, endpoint)

	// Whitespace-only endpoint
	endpoint, err = validateEndpoint("   ")
	assert.Error(t, err)
	assert.Nil(t, endpoint)
	assert.Contains(t, err.Error(), "endpoint is empty")

	// Endpoint with whitespace that should be trimmed
	endpoint, err = validateEndpoint("  http://169.254.170.2/v4  ")
	require.NoError(t, err)
	require.NotNil(t, endpoint)
	assert.Equal(t, "169.254.170.2", endpoint.Host)
}
