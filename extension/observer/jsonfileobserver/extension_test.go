// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonfileobserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestJSONFileObserver(t *testing.T) {
	t.Parallel()

	// Create a temporary JSON file
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "endpoints.json")

	initialEndpoints := []EndpointConfig{
		{
			ID:     "endpoint-1",
			Target: "localhost:8080",
			Name:   "Test Service 1",
			Labels: map[string]string{"env": "test"},
		},
		{
			ID:     "endpoint-2",
			Target: "localhost:9090",
			Name:   "Test Service 2",
			Labels: map[string]string{"env": "prod"},
		},
	}

	data, err := json.Marshal(initialEndpoints)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(jsonPath, data, 0600))

	// Create the observer
	cfg := &Config{
		Path:            jsonPath,
		RefreshInterval: 100 * time.Millisecond,
	}

	set := extensiontest.NewNopSettings(component.MustNewType("jsonfile_observer"))
	obs, err := newObserver(set, cfg)
	require.NoError(t, err)
	require.NotNil(t, obs)

	// Start the observer
	err = obs.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Allow time for initial load
	time.Sleep(200 * time.Millisecond)

	// Verify initial endpoints
	endpoints := obs.handler.ListEndpoints()
	assert.Len(t, endpoints, 2)

	// Verify endpoint content
	endpointMap := make(map[string]observer.Endpoint)
	for _, ep := range endpoints {
		endpointMap[ep.Target] = ep
	}

	ep1, ok := endpointMap["localhost:8080"]
	require.True(t, ok, "Expected endpoint with target localhost:8080")
	assert.Equal(t, "Test Service 1", ep1.Details.(*JSONFileEndpoint).Name)

	ep2, ok := endpointMap["localhost:9090"]
	require.True(t, ok, "Expected endpoint with target localhost:9090")
	assert.Equal(t, "Test Service 2", ep2.Details.(*JSONFileEndpoint).Name)

	// Shutdown
	err = obs.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestJSONFileObserverPicksUpFileChanges(t *testing.T) {
	t.Parallel()

	// Create a temporary JSON file
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "endpoints.json")

	// Initial endpoints
	initialEndpoints := []EndpointConfig{
		{
			ID:     "endpoint-1",
			Target: "localhost:8080",
			Name:   "Initial Service",
			Labels: map[string]string{"version": "v1"},
		},
	}

	data, err := json.Marshal(initialEndpoints)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(jsonPath, data, 0600))

	// Create the observer with a short refresh interval
	cfg := &Config{
		Path:            jsonPath,
		RefreshInterval: 100 * time.Millisecond,
	}

	set := extensiontest.NewNopSettings(component.MustNewType("jsonfile_observer"))
	obs, err := newObserver(set, cfg)
	require.NoError(t, err)

	// Start the observer
	err = obs.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		_ = obs.Shutdown(context.Background())
	}()

	// Wait for initial load
	time.Sleep(200 * time.Millisecond)

	// Verify initial state - 1 endpoint
	endpoints := obs.handler.ListEndpoints()
	require.Len(t, endpoints, 1, "Expected 1 endpoint initially")
	assert.Equal(t, "Initial Service", endpoints[0].Details.(*JSONFileEndpoint).Name)

	// Update the JSON file with new endpoints
	updatedEndpoints := []EndpointConfig{
		{
			ID:     "endpoint-1",
			Target: "localhost:8080",
			Name:   "Updated Service",
			Labels: map[string]string{"version": "v2"},
		},
		{
			ID:     "endpoint-new",
			Target: "localhost:3000",
			Name:   "New Service",
			Labels: map[string]string{"version": "v1"},
		},
	}

	// Wait a bit to ensure different modification time
	time.Sleep(50 * time.Millisecond)

	data, err = json.Marshal(updatedEndpoints)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(jsonPath, data, 0600))

	// Wait for the observer to pick up the changes
	time.Sleep(300 * time.Millisecond)

	// Verify updated state - 2 endpoints
	endpoints = obs.handler.ListEndpoints()
	require.Len(t, endpoints, 2, "Expected 2 endpoints after update")

	// Check the updated endpoint and the new one
	endpointMap := make(map[string]observer.Endpoint)
	for _, ep := range endpoints {
		endpointMap[ep.Target] = ep
	}

	ep1, ok := endpointMap["localhost:8080"]
	require.True(t, ok, "Expected endpoint with target localhost:8080")
	assert.Equal(t, "Updated Service", ep1.Details.(*JSONFileEndpoint).Name)
	assert.Equal(t, map[string]string{"version": "v2"}, ep1.Details.(*JSONFileEndpoint).Labels)

	epNew, ok := endpointMap["localhost:3000"]
	require.True(t, ok, "Expected new endpoint with target localhost:3000")
	assert.Equal(t, "New Service", epNew.Details.(*JSONFileEndpoint).Name)
}

func TestJSONFileObserverHandlesRemovedEndpoints(t *testing.T) {
	t.Parallel()

	// Create a temporary JSON file
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "endpoints.json")

	// Initial endpoints
	initialEndpoints := []EndpointConfig{
		{
			ID:     "endpoint-1",
			Target: "localhost:8080",
			Name:   "Service 1",
		},
		{
			ID:     "endpoint-2",
			Target: "localhost:9090",
			Name:   "Service 2",
		},
		{
			ID:     "endpoint-3",
			Target: "localhost:3000",
			Name:   "Service 3",
		},
	}

	data, err := json.Marshal(initialEndpoints)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(jsonPath, data, 0600))

	// Create the observer
	cfg := &Config{
		Path:            jsonPath,
		RefreshInterval: 100 * time.Millisecond,
	}

	set := extensiontest.NewNopSettings(component.MustNewType("jsonfile_observer"))
	obs, err := newObserver(set, cfg)
	require.NoError(t, err)

	// Start the observer
	err = obs.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		_ = obs.Shutdown(context.Background())
	}()

	// Wait for initial load
	time.Sleep(200 * time.Millisecond)

	// Verify initial state - 3 endpoints
	endpoints := obs.handler.ListEndpoints()
	require.Len(t, endpoints, 3, "Expected 3 endpoints initially")

	// Update the JSON file - remove endpoint-2
	updatedEndpoints := []EndpointConfig{
		{
			ID:     "endpoint-1",
			Target: "localhost:8080",
			Name:   "Service 1",
		},
		{
			ID:     "endpoint-3",
			Target: "localhost:3000",
			Name:   "Service 3",
		},
	}

	// Wait to ensure different modification time
	time.Sleep(50 * time.Millisecond)

	data, err = json.Marshal(updatedEndpoints)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(jsonPath, data, 0600))

	// Wait for the observer to pick up the changes
	time.Sleep(300 * time.Millisecond)

	// Verify updated state - 2 endpoints (endpoint-2 was removed)
	endpoints = obs.handler.ListEndpoints()
	require.Len(t, endpoints, 2, "Expected 2 endpoints after removal")

	// Verify endpoint-2 is gone
	for _, ep := range endpoints {
		assert.NotEqual(t, "localhost:9090", ep.Target, "endpoint-2 should have been removed")
	}
}

func TestJSONFileObserverInvalidJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "endpoints.json")

	// Write invalid JSON
	require.NoError(t, os.WriteFile(jsonPath, []byte("not valid json"), 0600))

	cfg := &Config{
		Path:            jsonPath,
		RefreshInterval: 100 * time.Millisecond,
	}

	set := extensiontest.NewNopSettings(component.MustNewType("jsonfile_observer"))
	obs, err := newObserver(set, cfg)
	require.NoError(t, err)

	// Start should not fail even with invalid JSON (just logs warning)
	err = obs.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Should have no endpoints
	endpoints := obs.handler.ListEndpoints()
	assert.Len(t, endpoints, 0)

	err = obs.Shutdown(context.Background())
	require.NoError(t, err)
}
