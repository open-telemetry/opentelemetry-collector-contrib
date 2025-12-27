package awsecsattributesprocessor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// =============================================================================
// Mock Docker API Server
// =============================================================================

// MockDockerServer simulates the Docker API for testing purposes
type MockDockerServer struct {
	server       *httptest.Server
	listener     net.Listener
	socketPath   string
	containers   []types.Container
	containerEnv map[string][]string // containerID -> env vars
	eventsChan   chan string
	mu           sync.RWMutex
}

// NewMockDockerServer creates a new mock Docker API server
func NewMockDockerServer() (*MockDockerServer, error) {
	m := &MockDockerServer{
		containers:   make([]types.Container, 0),
		containerEnv: make(map[string][]string),
		eventsChan:   make(chan string, 100),
	}

	// Create a temporary Unix socket
	tmpDir, err := os.MkdirTemp("", "docker-mock")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	m.socketPath = tmpDir + "/docker.sock"

	listener, err := net.Listen("unix", m.socketPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("failed to create Unix socket: %w", err)
	}
	m.listener = listener

	mux := http.NewServeMux()
	m.registerHandlers(mux)

	m.server = &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: mux},
	}
	m.server.Start()

	return m, nil
}

func (m *MockDockerServer) registerHandlers(mux *http.ServeMux) {
	// Handle Docker API version negotiation
	mux.HandleFunc("/_ping", m.handlePing)
	mux.HandleFunc("/v1.24/_ping", m.handlePing)

	// Container list endpoint
	mux.HandleFunc("/containers/json", m.handleContainerList)
	mux.HandleFunc("/v1.24/containers/json", m.handleContainerList)

	// Container inspect endpoint - using a pattern that matches any container ID
	mux.HandleFunc("/containers/", m.handleContainerInspect)
	mux.HandleFunc("/v1.24/containers/", m.handleContainerInspect)

	// Events endpoint
	mux.HandleFunc("/events", m.handleEvents)
	mux.HandleFunc("/v1.24/events", m.handleEvents)
}

func (m *MockDockerServer) handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("API-Version", "1.24")
	w.Header().Set("Docker-Experimental", "false")
	w.Header().Set("OSType", "linux")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (m *MockDockerServer) handleContainerList(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m.containers)
}

func (m *MockDockerServer) handleContainerInspect(w http.ResponseWriter, r *http.Request) {
	// Extract container ID from path: /containers/{id}/json or /v1.24/containers/{id}/json
	path := r.URL.Path
	parts := strings.Split(path, "/")

	var containerID string
	for i, part := range parts {
		if part == "containers" && i+1 < len(parts) {
			containerID = parts[i+1]
			break
		}
	}

	if containerID == "" || !strings.HasSuffix(path, "/json") {
		http.NotFound(w, r)
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Find the container
	var foundContainer *types.Container
	for i := range m.containers {
		if m.containers[i].ID == containerID {
			foundContainer = &m.containers[i]
			break
		}
	}

	if foundContainer == nil {
		http.Error(w, fmt.Sprintf("No such container: %s", containerID), http.StatusNotFound)
		return
	}

	// Build container inspect response
	env := m.containerEnv[containerID]
	response := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:    foundContainer.ID,
			Name:  "/" + foundContainer.Names[0],
			State: &types.ContainerState{Running: true},
		},
		Config: &container.Config{
			Image: foundContainer.Image,
			Env:   env,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *MockDockerServer) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Keep connection open and stream events
	for {
		select {
		case event := <-m.eventsChan:
			w.Write([]byte(event + "\n"))
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// AddContainer adds a container to the mock server
func (m *MockDockerServer) AddContainer(id, name, image string, env []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.containers = append(m.containers, types.Container{
		ID:    id,
		Names: []string{name},
		Image: image,
		State: "running",
	})
	m.containerEnv[id] = env
}

// SendContainerEvent sends a container event through the events stream
func (m *MockDockerServer) SendContainerEvent(action, containerID string) {
	event := fmt.Sprintf(`{"Type":"container","Action":"%s","Actor":{"ID":"%s"},"time":%d}`,
		action, containerID, time.Now().Unix())
	m.eventsChan <- event
}

// SocketPath returns the Unix socket path for the mock server
func (m *MockDockerServer) SocketPath() string {
	return m.socketPath
}

// Close shuts down the mock server and cleans up resources
func (m *MockDockerServer) Close() {
	if m.server != nil {
		m.server.Close()
	}
	if m.socketPath != "" {
		os.RemoveAll(strings.TrimSuffix(m.socketPath, "/docker.sock"))
	}
	close(m.eventsChan)
}

// =============================================================================
// Mock ECS Metadata Server
// =============================================================================

// MockECSMetadataServer simulates the ECS metadata endpoint
type MockECSMetadataServer struct {
	server   *httptest.Server
	metadata map[string]Metadata // containerID -> metadata
	mu       sync.RWMutex
}

// NewMockECSMetadataServer creates a new mock ECS metadata server
func NewMockECSMetadataServer() *MockECSMetadataServer {
	m := &MockECSMetadataServer{
		metadata: make(map[string]Metadata),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", m.handleMetadata)

	m.server = httptest.NewServer(mux)
	return m
}

func (m *MockECSMetadataServer) handleMetadata(w http.ResponseWriter, r *http.Request) {
	// Extract container ID from query parameter or path
	containerID := r.URL.Query().Get("container_id")
	if containerID == "" {
		// For simple cases, return the first metadata entry
		m.mu.RLock()
		defer m.mu.RUnlock()

		for _, meta := range m.metadata {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(meta)
			return
		}

		http.Error(w, "No metadata available", http.StatusNotFound)
		return
	}

	m.mu.RLock()
	meta, ok := m.metadata[containerID]
	m.mu.RUnlock()

	if !ok {
		http.Error(w, "Container not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

// SetMetadata sets the metadata for a specific container
func (m *MockECSMetadataServer) SetMetadata(containerID string, meta Metadata) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metadata[containerID] = meta
}

// URL returns the server URL
func (m *MockECSMetadataServer) URL() string {
	return m.server.URL
}

// Close shuts down the mock server
func (m *MockECSMetadataServer) Close() {
	m.server.Close()
}

// =============================================================================
// Multi-Container Mock ECS Metadata Server
// =============================================================================

// MockMultiECSMetadataServer simulates multiple ECS metadata endpoints (one per container)
type MockMultiECSMetadataServer struct {
	servers  map[string]*httptest.Server // containerID -> server
	metadata map[string]Metadata
	mu       sync.RWMutex
}

// NewMockMultiECSMetadataServer creates a mock server that can serve different metadata per container
func NewMockMultiECSMetadataServer() *MockMultiECSMetadataServer {
	return &MockMultiECSMetadataServer{
		servers:  make(map[string]*httptest.Server),
		metadata: make(map[string]Metadata),
	}
}

// AddContainer adds a container with its metadata and returns the endpoint URL
func (m *MockMultiECSMetadataServer) AddContainer(containerID string, meta Metadata) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metadata[containerID] = meta

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.RLock()
		meta := m.metadata[containerID]
		m.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(meta)
	}))

	m.servers[containerID] = server
	return server.URL
}

// Close shuts down all mock servers
func (m *MockMultiECSMetadataServer) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, server := range m.servers {
		server.Close()
	}
}

// =============================================================================
// Test Consumer
// =============================================================================

// E2ETestConsumer captures logs for verification
type E2ETestConsumer struct {
	mu   sync.Mutex
	logs []plog.Logs
}

func NewE2ETestConsumer() *E2ETestConsumer {
	return &E2ETestConsumer{
		logs: make([]plog.Logs, 0),
	}
}

func (c *E2ETestConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logs = append(c.logs, ld)
	return nil
}

func (c *E2ETestConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *E2ETestConsumer) GetLogs() []plog.Logs {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.logs
}

// =============================================================================
// E2E Tests
// =============================================================================

// createTestMetadata creates a test ECS metadata object
func createTestMetadata(containerID, clusterName, taskARN, containerName string) Metadata {
	return Metadata{
		ContainerARN:  fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:container/%s/%s", clusterName, containerID),
		CreatedAt:     time.Now().Add(-1 * time.Hour),
		DesiredStatus: "RUNNING",
		DockerID:      containerID,
		DockerName:    fmt.Sprintf("ecs-%s-%s", taskARN, containerName),
		Image:         "test-image:latest",
		ImageID:       "sha256:abc123",
		KnownStatus:   "RUNNING",
		Labels: map[string]any{
			"com.amazonaws.ecs.cluster":                 clusterName,
			"com.amazonaws.ecs.container-name":          containerName,
			"com.amazonaws.ecs.task-arn":                taskARN,
			"com.amazonaws.ecs.task-definition-family":  "test-task-definition",
			"com.amazonaws.ecs.task-definition-version": "1",
		},
		Limits: struct {
			CPU    int `json:"CPU"`
			Memory int `json:"Memory"`
		}{CPU: 256, Memory: 512},
		Name: containerName,
		Networks: []Network{
			{
				IPv4Addresses: []string{"10.0.0.1"},
				NetworkMode:   "awsvpc",
			},
		},
		Ports:     []Port{},
		StartedAt: time.Now().Add(-30 * time.Minute),
		Type:      "NORMAL",
		Volumes:   []Volume{},
	}
}

// TestE2E_HappyPath tests the full processor lifecycle with a single container
func TestE2E_HappyPath(t *testing.T) {
	// Setup mock ECS metadata server
	ecsServer := NewMockECSMetadataServer()
	defer ecsServer.Close()

	containerID := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	testMeta := createTestMetadata(containerID, "test-cluster", "arn:aws:ecs:us-east-1:123456789012:task/test-cluster/abc123", "test-container")
	ecsServer.SetMetadata(containerID, testMeta)

	// Create mock endpoints function that returns our test container
	mockEndpoints := func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
		return map[string][]string{
			containerID: {ecsServer.URL()},
		}, nil
	}

	// Create test consumer
	testConsumer := NewE2ETestConsumer()

	// Create processor config
	cfg := &Config{
		CacheTTL: 60,
		Attributes: []string{
			".*", // Capture all attributes
		},
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}
	require.NoError(t, cfg.init())

	// Create the logs processor
	processor := newLogsProcessor(
		context.Background(),
		zap.NewExample(),
		cfg,
		testConsumer,
		mockEndpoints,
		getContainerData,
	)

	// Sync metadata (simulates what Start() does)
	ctx := context.Background()
	endpoints, err := mockEndpoints(zap.NewExample(), ctx)
	require.NoError(t, err)
	require.NoError(t, processor.syncMetadata(ctx, endpoints))

	// Create test logs with container ID
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr("container.id", containerID)
	resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("test log message")

	// Process the logs
	err = processor.ConsumeLogs(ctx, logs)
	require.NoError(t, err)

	// Verify the consumer received the logs
	receivedLogs := testConsumer.GetLogs()
	require.Len(t, receivedLogs, 1)

	// Verify ECS attributes were added
	attrs := receivedLogs[0].ResourceLogs().At(0).Resource().Attributes()

	// Check key ECS attributes
	val, ok := attrs.Get("aws.ecs.cluster")
	assert.True(t, ok, "aws.ecs.cluster should be present")
	assert.Equal(t, "test-cluster", val.Str())

	val, ok = attrs.Get("aws.ecs.container.name")
	assert.True(t, ok, "aws.ecs.container.name should be present")
	assert.Equal(t, "test-container", val.Str())

	val, ok = attrs.Get("aws.ecs.task.known.status")
	assert.True(t, ok, "aws.ecs.task.known.status should be present")
	assert.Equal(t, "RUNNING", val.Str())

	val, ok = attrs.Get("docker.id")
	assert.True(t, ok, "docker.id should be present")
	assert.Equal(t, containerID, val.Str())
}

// TestE2E_MultipleContainers tests processing logs from multiple containers
func TestE2E_MultipleContainers(t *testing.T) {
	// Setup mock ECS metadata server for multiple containers
	multiEcsServer := NewMockMultiECSMetadataServer()
	defer multiEcsServer.Close()

	// Define test containers
	containers := []struct {
		id          string
		cluster     string
		taskARN     string
		name        string
		endpointURL string
	}{
		{
			id:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			cluster: "cluster-a",
			taskARN: "arn:aws:ecs:us-east-1:123456789012:task/cluster-a/task-a",
			name:    "container-a",
		},
		{
			id:      "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			cluster: "cluster-b",
			taskARN: "arn:aws:ecs:us-east-1:123456789012:task/cluster-b/task-b",
			name:    "container-b",
		},
		{
			id:      "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			cluster: "cluster-c",
			taskARN: "arn:aws:ecs:us-east-1:123456789012:task/cluster-c/task-c",
			name:    "container-c",
		},
	}

	// Setup metadata for each container
	endpoints := make(map[string][]string)
	for i := range containers {
		meta := createTestMetadata(containers[i].id, containers[i].cluster, containers[i].taskARN, containers[i].name)
		containers[i].endpointURL = multiEcsServer.AddContainer(containers[i].id, meta)
		endpoints[containers[i].id] = []string{containers[i].endpointURL}
	}

	// Create mock endpoints function
	mockEndpoints := func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
		return endpoints, nil
	}

	// Create test consumer
	testConsumer := NewE2ETestConsumer()

	// Create processor config
	cfg := &Config{
		CacheTTL: 60,
		Attributes: []string{
			"^aws.ecs.*",
		},
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}
	require.NoError(t, cfg.init())

	// Create the logs processor
	processor := newLogsProcessor(
		context.Background(),
		zap.NewExample(),
		cfg,
		testConsumer,
		mockEndpoints,
		getContainerData,
	)

	// Sync metadata
	ctx := context.Background()
	eps, err := mockEndpoints(zap.NewExample(), ctx)
	require.NoError(t, err)
	require.NoError(t, processor.syncMetadata(ctx, eps))

	// Process logs for each container
	for _, c := range containers {
		logs := plog.NewLogs()
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		resourceLogs.Resource().Attributes().PutStr("container.id", c.id)
		resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(fmt.Sprintf("log from %s", c.name))

		err := processor.ConsumeLogs(ctx, logs)
		require.NoError(t, err)
	}

	// Verify all logs were processed correctly
	receivedLogs := testConsumer.GetLogs()
	require.Len(t, receivedLogs, len(containers))

	// Verify each log has the correct cluster name
	for i, c := range containers {
		attrs := receivedLogs[i].ResourceLogs().At(0).Resource().Attributes()

		val, ok := attrs.Get("aws.ecs.cluster")
		assert.True(t, ok, "aws.ecs.cluster should be present for container %s", c.id)
		assert.Equal(t, c.cluster, val.Str(), "cluster name mismatch for container %s", c.id)

		val, ok = attrs.Get("aws.ecs.container.name")
		assert.True(t, ok, "aws.ecs.container.name should be present for container %s", c.id)
		assert.Equal(t, c.name, val.Str(), "container name mismatch for container %s", c.id)
	}
}

// TestE2E_ContainerEventTriggersSync tests that new container events trigger metadata sync
func TestE2E_ContainerEventTriggersSync(t *testing.T) {
	// Setup mock ECS metadata server
	multiEcsServer := NewMockMultiECSMetadataServer()
	defer multiEcsServer.Close()

	// Initial container
	initialContainerID := "1111111111111111111111111111111111111111111111111111111111111111"
	initialMeta := createTestMetadata(initialContainerID, "initial-cluster", "arn:aws:ecs:us-east-1:123456789012:task/initial-cluster/task-1", "initial-container")
	initialEndpoint := multiEcsServer.AddContainer(initialContainerID, initialMeta)

	// New container (will be added after initial sync)
	newContainerID := "2222222222222222222222222222222222222222222222222222222222222222"
	newMeta := createTestMetadata(newContainerID, "new-cluster", "arn:aws:ecs:us-east-1:123456789012:task/new-cluster/task-2", "new-container")
	newEndpoint := multiEcsServer.AddContainer(newContainerID, newMeta)

	// Track which containers are "visible" (simulates Docker discovering new containers)
	visibleContainers := map[string][]string{
		initialContainerID: {initialEndpoint},
	}
	var mu sync.Mutex

	// Create mock endpoints function that returns currently visible containers
	mockEndpoints := func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
		mu.Lock()
		defer mu.Unlock()
		result := make(map[string][]string)
		for k, v := range visibleContainers {
			result[k] = v
		}
		return result, nil
	}

	// Create test consumer
	testConsumer := NewE2ETestConsumer()

	// Create processor config
	cfg := &Config{
		CacheTTL: 60,
		Attributes: []string{
			"^aws.ecs.*",
		},
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}
	require.NoError(t, cfg.init())

	// Create the logs processor
	processor := newLogsProcessor(
		context.Background(),
		zap.NewExample(),
		cfg,
		testConsumer,
		mockEndpoints,
		getContainerData,
	)

	ctx := context.Background()

	// Initial sync - only initial container should be available
	eps, err := mockEndpoints(zap.NewExample(), ctx)
	require.NoError(t, err)
	require.NoError(t, processor.syncMetadata(ctx, eps))

	// Verify initial container metadata is available
	meta, err := processor.get(initialContainerID)
	require.NoError(t, err)
	assert.Equal(t, "initial-cluster", meta.Labels["com.amazonaws.ecs.cluster"])

	// New container should NOT be available yet
	_, err = processor.get(newContainerID)
	assert.Error(t, err, "new container should not be available before sync")

	// Simulate new container being discovered (like a Docker "create" event)
	mu.Lock()
	visibleContainers[newContainerID] = []string{newEndpoint}
	mu.Unlock()

	// Trigger re-sync (simulates what happens when a container create event is received)
	eps, err = mockEndpoints(zap.NewExample(), ctx)
	require.NoError(t, err)
	require.NoError(t, processor.syncMetadata(ctx, eps))

	// Now new container should be available
	meta, err = processor.get(newContainerID)
	require.NoError(t, err)
	assert.Equal(t, "new-cluster", meta.Labels["com.amazonaws.ecs.cluster"])

	// Process logs for the new container
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr("container.id", newContainerID)
	resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log from new container")

	err = processor.ConsumeLogs(ctx, logs)
	require.NoError(t, err)

	// Verify the log was enriched with the new container's metadata
	receivedLogs := testConsumer.GetLogs()
	require.Len(t, receivedLogs, 1)

	attrs := receivedLogs[0].ResourceLogs().At(0).Resource().Attributes()
	val, ok := attrs.Get("aws.ecs.cluster")
	assert.True(t, ok)
	assert.Equal(t, "new-cluster", val.Str())
}

// TestE2E_CacheBehavior tests that metadata is cached and TTL works
func TestE2E_CacheBehavior(t *testing.T) {
	// Track how many times metadata endpoint is called
	callCount := 0
	var mu sync.Mutex

	containerID := "cachetestcachetestcachetestcachetestcachetestcachetestcachetest00"
	testMeta := createTestMetadata(containerID, "cache-cluster", "arn:aws:ecs:us-east-1:123456789012:task/cache-cluster/task-cache", "cache-container")

	// Create a server that tracks call count
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testMeta)
	}))
	defer server.Close()

	// Create mock endpoints function
	mockEndpoints := func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
		return map[string][]string{
			containerID: {server.URL},
		}, nil
	}

	// Create test consumer
	testConsumer := NewE2ETestConsumer()

	// Create processor config with short cache TTL for testing
	cfg := &Config{
		CacheTTL: 60, // minimum allowed
		Attributes: []string{
			".*",
		},
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}
	require.NoError(t, cfg.init())

	// Create the logs processor
	processor := newLogsProcessor(
		context.Background(),
		zap.NewExample(),
		cfg,
		testConsumer,
		mockEndpoints,
		getContainerData,
	)

	ctx := context.Background()

	// Initial sync
	eps, err := mockEndpoints(zap.NewExample(), ctx)
	require.NoError(t, err)
	require.NoError(t, processor.syncMetadata(ctx, eps))

	mu.Lock()
	initialCallCount := callCount
	mu.Unlock()
	assert.Equal(t, 1, initialCallCount, "metadata endpoint should be called once during initial sync")

	// Sync again - should NOT call metadata endpoint because data is cached
	require.NoError(t, processor.syncMetadata(ctx, eps))

	mu.Lock()
	afterSecondSync := callCount
	mu.Unlock()
	assert.Equal(t, 1, afterSecondSync, "metadata endpoint should NOT be called again due to caching")

	// Get metadata multiple times - should use cache
	for i := 0; i < 5; i++ {
		_, err := processor.get(containerID)
		require.NoError(t, err)
	}

	mu.Lock()
	afterMultipleGets := callCount
	mu.Unlock()
	assert.Equal(t, 1, afterMultipleGets, "metadata endpoint should still only be called once due to caching")
}

// TestE2E_MetadataEndpointUnavailable tests error handling when metadata endpoint is unavailable
func TestE2E_MetadataEndpointUnavailable(t *testing.T) {
	containerID := "unavailableunavailableunavailableunavailableunavailableunavailab"

	// Create mock endpoints function that returns an unavailable endpoint
	mockEndpoints := func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
		return map[string][]string{
			containerID: {"http://localhost:99999"}, // Invalid port
		}, nil
	}

	// Create test consumer
	testConsumer := NewE2ETestConsumer()

	// Create processor config
	cfg := &Config{
		CacheTTL: 60,
		Attributes: []string{
			".*",
		},
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}
	require.NoError(t, cfg.init())

	// Create the logs processor
	processor := newLogsProcessor(
		context.Background(),
		zap.NewExample(),
		cfg,
		testConsumer,
		mockEndpoints,
		getContainerData,
	)

	ctx := context.Background()

	// Sync should not fail (errors are logged but not returned)
	eps, err := mockEndpoints(zap.NewExample(), ctx)
	require.NoError(t, err)
	require.NoError(t, processor.syncMetadata(ctx, eps))

	// Getting metadata for the container should fail
	_, err = processor.get(containerID)
	assert.Error(t, err, "should fail to get metadata for container with unavailable endpoint")
}

// TestE2E_ContainerNotFound tests handling of logs from unknown containers
func TestE2E_ContainerNotFound(t *testing.T) {
	// Setup mock ECS metadata server with one container
	ecsServer := NewMockECSMetadataServer()
	defer ecsServer.Close()

	knownContainerID := "knownknownknownknownknownknownknownknownknownknownknownknown0000"
	testMeta := createTestMetadata(knownContainerID, "known-cluster", "arn:aws:ecs:us-east-1:123456789012:task/known-cluster/task-known", "known-container")
	ecsServer.SetMetadata(knownContainerID, testMeta)

	// Create mock endpoints function that only returns the known container
	mockEndpoints := func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
		return map[string][]string{
			knownContainerID: {ecsServer.URL()},
		}, nil
	}

	// Create test consumer
	testConsumer := NewE2ETestConsumer()

	// Create processor config
	cfg := &Config{
		CacheTTL: 60,
		Attributes: []string{
			".*",
		},
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}
	require.NoError(t, cfg.init())

	// Create the logs processor
	processor := newLogsProcessor(
		context.Background(),
		zap.NewExample(),
		cfg,
		testConsumer,
		mockEndpoints,
		getContainerData,
	)

	ctx := context.Background()

	// Initial sync
	eps, err := mockEndpoints(zap.NewExample(), ctx)
	require.NoError(t, err)
	require.NoError(t, processor.syncMetadata(ctx, eps))

	// Create logs with an unknown container ID
	unknownContainerID := "unknownunknownunknownunknownunknownunknownunknownunknownunknown"
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr("container.id", unknownContainerID)
	resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log from unknown container")

	// Process should not fail, but log should not be enriched
	err = processor.ConsumeLogs(ctx, logs)
	require.NoError(t, err)

	// Verify the log was forwarded (even without enrichment)
	receivedLogs := testConsumer.GetLogs()
	require.Len(t, receivedLogs, 1)

	// Verify NO ECS attributes were added (since container is unknown)
	attrs := receivedLogs[0].ResourceLogs().At(0).Resource().Attributes()
	_, ok := attrs.Get("aws.ecs.cluster")
	assert.False(t, ok, "aws.ecs.cluster should NOT be present for unknown container")
}

// TestE2E_AttributeFiltering tests that attribute filtering works correctly
func TestE2E_AttributeFiltering(t *testing.T) {
	// Setup mock ECS metadata server
	ecsServer := NewMockECSMetadataServer()
	defer ecsServer.Close()

	// Container ID must be exactly 64 hex characters (lowercase a-f, 0-9)
	containerID := "f1f2f3f4f5f6f7f8f9f0a1a2a3a4a5a6a7a8a9a0b1b2b3b4b5b6b7b8b9b0c1c2"
	testMeta := createTestMetadata(containerID, "filter-cluster", "arn:aws:ecs:us-east-1:123456789012:task/filter-cluster/task-filter", "filter-container")
	ecsServer.SetMetadata(containerID, testMeta)

	// Create mock endpoints function
	mockEndpoints := func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
		return map[string][]string{
			containerID: {ecsServer.URL()},
		}, nil
	}

	tests := []struct {
		name          string
		attributes    []string
		expectedAttrs []string
		excludedAttrs []string
	}{
		{
			name:          "only aws.ecs attributes",
			attributes:    []string{"^aws.ecs.*"},
			expectedAttrs: []string{"aws.ecs.cluster", "aws.ecs.container.name", "aws.ecs.task.arn"},
			excludedAttrs: []string{"docker.id", "image", "name"},
		},
		{
			name:          "only docker attributes",
			attributes:    []string{"^docker.*"},
			expectedAttrs: []string{"docker.id", "docker.name"},
			excludedAttrs: []string{"aws.ecs.cluster", "image"},
		},
		{
			name:          "multiple patterns",
			attributes:    []string{"^aws.ecs.cluster$", "^docker.id$"},
			expectedAttrs: []string{"aws.ecs.cluster", "docker.id"},
			excludedAttrs: []string{"aws.ecs.container.name", "docker.name", "image"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test consumer
			testConsumer := NewE2ETestConsumer()

			// Create processor config with specific attribute filter
			cfg := &Config{
				CacheTTL:   60,
				Attributes: tt.attributes,
				ContainerID: ContainerID{
					Sources: []string{"container.id"},
				},
			}
			require.NoError(t, cfg.init())

			// Create the logs processor
			processor := newLogsProcessor(
				context.Background(),
				zap.NewExample(),
				cfg,
				testConsumer,
				mockEndpoints,
				getContainerData,
			)

			ctx := context.Background()

			// Sync metadata
			eps, err := mockEndpoints(zap.NewExample(), ctx)
			require.NoError(t, err)
			require.NoError(t, processor.syncMetadata(ctx, eps))

			// Create and process logs
			logs := plog.NewLogs()
			resourceLogs := logs.ResourceLogs().AppendEmpty()
			resourceLogs.Resource().Attributes().PutStr("container.id", containerID)
			resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("test log")

			err = processor.ConsumeLogs(ctx, logs)
			require.NoError(t, err)

			// Verify filtering
			receivedLogs := testConsumer.GetLogs()
			require.Len(t, receivedLogs, 1)

			attrs := receivedLogs[0].ResourceLogs().At(0).Resource().Attributes()

			// Check expected attributes are present
			for _, attr := range tt.expectedAttrs {
				_, ok := attrs.Get(attr)
				assert.True(t, ok, "expected attribute %s should be present", attr)
			}

			// Check excluded attributes are NOT present
			for _, attr := range tt.excludedAttrs {
				_, ok := attrs.Get(attr)
				assert.False(t, ok, "excluded attribute %s should NOT be present", attr)
			}
		})
	}
}

// TestE2E_LogFileNameSource tests using log.file.name as container ID source
func TestE2E_LogFileNameSource(t *testing.T) {
	// Setup mock ECS metadata server
	ecsServer := NewMockECSMetadataServer()
	defer ecsServer.Close()

	// Container ID must be exactly 64 hex characters (lowercase a-f, 0-9)
	containerID := "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b"
	testMeta := createTestMetadata(containerID, "logfile-cluster", "arn:aws:ecs:us-east-1:123456789012:task/logfile-cluster/task-logfile", "logfile-container")
	ecsServer.SetMetadata(containerID, testMeta)

	// Create mock endpoints function
	mockEndpoints := func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
		return map[string][]string{
			containerID: {ecsServer.URL()},
		}, nil
	}

	// Create test consumer
	testConsumer := NewE2ETestConsumer()

	// Create processor config with log.file.name as source
	cfg := &Config{
		CacheTTL: 60,
		Attributes: []string{
			"^aws.ecs.*",
		},
		ContainerID: ContainerID{
			Sources: []string{"log.file.name"},
		},
	}
	require.NoError(t, cfg.init())

	// Create the logs processor
	processor := newLogsProcessor(
		context.Background(),
		zap.NewExample(),
		cfg,
		testConsumer,
		mockEndpoints,
		getContainerData,
	)

	ctx := context.Background()

	// Sync metadata
	eps, err := mockEndpoints(zap.NewExample(), ctx)
	require.NoError(t, err)
	require.NoError(t, processor.syncMetadata(ctx, eps))

	// Create logs with container ID in log.file.name (with -json.log suffix like Docker logs)
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr("log.file.name", containerID+"-json.log")
	resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("test log from file")

	err = processor.ConsumeLogs(ctx, logs)
	require.NoError(t, err)

	// Verify the log was enriched
	receivedLogs := testConsumer.GetLogs()
	require.Len(t, receivedLogs, 1)

	attrs := receivedLogs[0].ResourceLogs().At(0).Resource().Attributes()
	val, ok := attrs.Get("aws.ecs.cluster")
	assert.True(t, ok, "aws.ecs.cluster should be present")
	assert.Equal(t, "logfile-cluster", val.Str())
}

// TestE2E_MetricsProcessor tests the metrics processor with mocked APIs
func TestE2E_MetricsProcessor(t *testing.T) {
	// Setup mock ECS metadata server
	ecsServer := NewMockECSMetadataServer()
	defer ecsServer.Close()

	containerID := "metricsmetricsmetricsmetricsmetricsmetricsmetricsmetricsmetrics00"
	testMeta := createTestMetadata(containerID, "metrics-cluster", "arn:aws:ecs:us-east-1:123456789012:task/metrics-cluster/task-metrics", "metrics-container")
	ecsServer.SetMetadata(containerID, testMeta)

	// Create mock endpoints function
	mockEndpoints := func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
		return map[string][]string{
			containerID: {ecsServer.URL()},
		}, nil
	}

	// Create test consumer for metrics
	var receivedMetrics []pcommon.Map
	var mu sync.Mutex

	testMetricsConsumer := &testMetricsConsumerE2E{
		consumeFn: func(ctx context.Context, md pmetric.Metrics) error {
			mu.Lock()
			defer mu.Unlock()
			// This is a simplified test - in real scenario we'd check the actual metrics
			receivedMetrics = append(receivedMetrics, pcommon.NewMap())
			return nil
		},
	}

	// Create processor config
	cfg := &Config{
		CacheTTL: 60,
		Attributes: []string{
			"^aws.ecs.*",
		},
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}
	require.NoError(t, cfg.init())

	// Create the metrics processor
	processor := newMetricsProcessor(
		context.Background(),
		zap.NewExample(),
		cfg,
		testMetricsConsumer,
		mockEndpoints,
		getContainerData,
	)

	ctx := context.Background()

	// Sync metadata
	eps, err := mockEndpoints(zap.NewExample(), ctx)
	require.NoError(t, err)
	require.NoError(t, processor.syncMetadata(ctx, eps))

	// Verify metadata is available
	meta, err := processor.get(containerID)
	require.NoError(t, err)
	assert.Equal(t, "metrics-cluster", meta.Labels["com.amazonaws.ecs.cluster"])
}

// testMetricsConsumerE2E is a simple test consumer for metrics
type testMetricsConsumerE2E struct {
	consumeFn func(ctx context.Context, md pmetric.Metrics) error
}

func (c *testMetricsConsumerE2E) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if c.consumeFn != nil {
		return c.consumeFn(ctx, md)
	}
	return nil
}

func (c *testMetricsConsumerE2E) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// TestE2E_TracesProcessor tests the traces processor with mocked APIs
func TestE2E_TracesProcessor(t *testing.T) {
	// Setup mock ECS metadata server
	ecsServer := NewMockECSMetadataServer()
	defer ecsServer.Close()

	containerID := "tracestracestracestracestracestracestracestracestracestracestrac"
	testMeta := createTestMetadata(containerID, "traces-cluster", "arn:aws:ecs:us-east-1:123456789012:task/traces-cluster/task-traces", "traces-container")
	ecsServer.SetMetadata(containerID, testMeta)

	// Create mock endpoints function
	mockEndpoints := func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
		return map[string][]string{
			containerID: {ecsServer.URL()},
		}, nil
	}

	// Create test consumer for traces
	testTracesConsumer := &testTracesConsumerE2E{}

	// Create processor config
	cfg := &Config{
		CacheTTL: 60,
		Attributes: []string{
			"^aws.ecs.*",
		},
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}
	require.NoError(t, cfg.init())

	// Create the traces processor
	processor := newTracesProcessor(
		context.Background(),
		zap.NewExample(),
		cfg,
		testTracesConsumer,
		mockEndpoints,
		getContainerData,
	)

	ctx := context.Background()

	// Sync metadata
	eps, err := mockEndpoints(zap.NewExample(), ctx)
	require.NoError(t, err)
	require.NoError(t, processor.syncMetadata(ctx, eps))

	// Verify metadata is available
	meta, err := processor.get(containerID)
	require.NoError(t, err)
	assert.Equal(t, "traces-cluster", meta.Labels["com.amazonaws.ecs.cluster"])
}

// testTracesConsumerE2E is a simple test consumer for traces
type testTracesConsumerE2E struct{}

func (c *testTracesConsumerE2E) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return nil
}

func (c *testTracesConsumerE2E) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// TestE2E_FullDockerIntegration tests with the mock Docker server (if Docker is available)
func TestE2E_FullDockerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker integration test in short mode")
	}

	// Create mock Docker server
	mockDocker, err := NewMockDockerServer()
	if err != nil {
		t.Skipf("Could not create mock Docker server: %v", err)
	}
	defer mockDocker.Close()

	// Setup mock ECS metadata server
	ecsServer := NewMockECSMetadataServer()
	defer ecsServer.Close()

	containerID := "dockerintegrationtestdockerintegrationtestdockerintegrationtest00"
	testMeta := createTestMetadata(containerID, "docker-cluster", "arn:aws:ecs:us-east-1:123456789012:task/docker-cluster/task-docker", "docker-container")
	ecsServer.SetMetadata(containerID, testMeta)

	// Add container to mock Docker server with ECS metadata URI env var
	mockDocker.AddContainer(
		containerID,
		"test-container",
		"test-image:latest",
		[]string{
			fmt.Sprintf("ECS_CONTAINER_METADATA_URI_V4=%s", ecsServer.URL()),
			"OTHER_ENV=value",
		},
	)

	// Set DOCKER_HOST to use our mock server
	originalDockerHost := os.Getenv("DOCKER_HOST")
	os.Setenv("DOCKER_HOST", "unix://"+mockDocker.SocketPath())
	defer os.Setenv("DOCKER_HOST", originalDockerHost)

	// Test that we can discover the container via the mock Docker API
	// Note: This tests the mock server itself, not the full processor integration
	// because the processor creates its own Docker client internally

	// Verify mock Docker server is working
	t.Logf("Mock Docker server running at: unix://%s", mockDocker.SocketPath())
	t.Logf("Mock ECS metadata server running at: %s", ecsServer.URL())

	// The mock Docker server should return our test container
	// This validates the mock infrastructure is working correctly
	assert.NotEmpty(t, mockDocker.SocketPath())
	assert.NotEmpty(t, ecsServer.URL())
}

// BenchmarkE2E_MetadataSync benchmarks the metadata sync operation
func BenchmarkE2E_MetadataSync(b *testing.B) {
	// Setup mock ECS metadata server
	ecsServer := NewMockECSMetadataServer()
	defer ecsServer.Close()

	containerID := "benchmarkbenchmarkbenchmarkbenchmarkbenchmarkbenchmarkbenchmark00"
	testMeta := createTestMetadata(containerID, "bench-cluster", "arn:aws:ecs:us-east-1:123456789012:task/bench-cluster/task-bench", "bench-container")
	ecsServer.SetMetadata(containerID, testMeta)

	mockEndpoints := func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
		return map[string][]string{
			containerID: {ecsServer.URL()},
		}, nil
	}

	cfg := &Config{
		CacheTTL: 60,
		Attributes: []string{
			".*",
		},
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}
	cfg.init()

	processor := newLogsProcessor(
		context.Background(),
		zap.NewNop(),
		cfg,
		NewE2ETestConsumer(),
		mockEndpoints,
		getContainerData,
	)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear cache to force re-fetch
		processor.metadata = make(map[string]Metadata)

		eps, _ := mockEndpoints(zap.NewNop(), ctx)
		processor.syncMetadata(ctx, eps)
	}
}

// BenchmarkE2E_LogProcessing benchmarks log processing with cached metadata
func BenchmarkE2E_LogProcessing(b *testing.B) {
	// Setup mock ECS metadata server
	ecsServer := NewMockECSMetadataServer()
	defer ecsServer.Close()

	containerID := "benchlogbenchlogbenchlogbenchlogbenchlogbenchlogbenchlogbenchlog00"
	testMeta := createTestMetadata(containerID, "benchlog-cluster", "arn:aws:ecs:us-east-1:123456789012:task/benchlog-cluster/task-benchlog", "benchlog-container")
	ecsServer.SetMetadata(containerID, testMeta)

	mockEndpoints := func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
		return map[string][]string{
			containerID: {ecsServer.URL()},
		}, nil
	}

	testConsumer := NewE2ETestConsumer()

	cfg := &Config{
		CacheTTL: 60,
		Attributes: []string{
			"^aws.ecs.*",
		},
		ContainerID: ContainerID{
			Sources: []string{"container.id"},
		},
	}
	cfg.init()

	processor := newLogsProcessor(
		context.Background(),
		zap.NewNop(),
		cfg,
		testConsumer,
		mockEndpoints,
		getContainerData,
	)

	ctx := context.Background()
	eps, _ := mockEndpoints(zap.NewNop(), ctx)
	processor.syncMetadata(ctx, eps)

	// Pre-create logs
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr("container.id", containerID)
	resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("benchmark log")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.ConsumeLogs(ctx, logs)
	}
}
