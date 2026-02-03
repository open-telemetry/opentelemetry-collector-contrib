// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package metrics

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	collectormetrics "go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"google.golang.org/grpc"
)

// mockMetricsReceiver is a mock gRPC receiver for testing
type mockMetricsReceiver struct {
	collectormetrics.UnimplementedGRPCServer
	mu      sync.RWMutex
	metrics []pmetric.Metrics
}

func (m *mockMetricsReceiver) Export(_ context.Context, req collectormetrics.ExportRequest) (collectormetrics.ExportResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = append(m.metrics, req.Metrics())
	// Create a properly initialized response using the factory method
	return collectormetrics.NewExportResponse(), nil
}

func (m *mockMetricsReceiver) GetMetrics() []pmetric.Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics
}

func (m *mockMetricsReceiver) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = nil
}

// startMockReceiver starts a mock gRPC receiver on a local TCP port
func startMockReceiver(t *testing.T) (*grpc.Server, string, *mockMetricsReceiver) {
	// Find an available port
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err, "Failed to create listener")

	port := lis.Addr().(*net.TCPAddr).Port
	lis.Close() // Close the listener to free the port

	// Create a new listener on the same port
	lis, err = net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	require.NoError(t, err, "Failed to create listener on port %d", port)

	server := grpc.NewServer()
	receiver := &mockMetricsReceiver{}
	collectormetrics.RegisterGRPCServer(server, receiver)

	go func() {
		if err := server.Serve(lis); err != nil {
			t.Logf("Server failed to serve: %v", err)
		}
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	return server, fmt.Sprintf("localhost:%d", port), receiver
}

// TestTelemetrygenIntegration tests the actual behavior of the telemetrygen tool
func TestTelemetrygenIntegration(t *testing.T) {
	// Create unique binary name to avoid conflicts between tests
	binaryName := fmt.Sprintf("telemetrygen-test-%d", time.Now().UnixNano())
	buildDir := "../../../telemetrygen"
	buildCmd := exec.Command("go", "build", "-o", binaryName, ".")
	buildCmd.Dir = buildDir
	err := buildCmd.Run()
	require.NoError(t, err, "Failed to build telemetrygen")

	defer os.Remove("../../../telemetrygen/" + binaryName)

	testBinaryPath := "../../../telemetrygen/" + binaryName

	t.Run("RespectsMetricsParameterWithBatching", func(t *testing.T) {
		server, endpoint, receiver := startMockReceiver(t)
		defer func() {
			server.Stop()
			// Wait for server to fully stop and connections to close
			time.Sleep(200 * time.Millisecond)
		}()

		// Reset receiver to ensure clean state
		receiver.Reset()

		// Add timeout to prevent hanging
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, testBinaryPath, "metrics", "--metrics", "5", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure", "--batch", "--batch-size", "2")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with batching enabled")
		assert.Less(t, duration, 6*time.Second, "Should complete quickly without connection issues")

		// Wait for all metrics to be processed
		time.Sleep(500 * time.Millisecond)
		metrics := receiver.GetMetrics()

		assert.Len(t, metrics, 3, "Should have received exactly 3 metric batches with batching enabled")
	})

	t.Run("RespectsMetricsParameterWithoutBatching", func(t *testing.T) {
		server, endpoint, receiver := startMockReceiver(t)
		defer func() {
			server.Stop()
			// Wait for server to fully stop and connections to close
			time.Sleep(200 * time.Millisecond)
		}()

		// Reset receiver to ensure clean state
		receiver.Reset()

		// Add timeout to prevent hanging
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, testBinaryPath, "metrics", "--metrics", "4", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure", "--batch=false")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with batching disabled")
		assert.Less(t, duration, 6*time.Second, "Should complete quickly without connection issues")

		// Wait for all metrics to be processed
		time.Sleep(500 * time.Millisecond)
		metrics := receiver.GetMetrics()

		assert.Len(t, metrics, 4, "Should have received exactly 4 metric batches with batching disabled")
	})

	t.Run("DurationOverridesMetrics", func(t *testing.T) {
		server, endpoint, receiver := startMockReceiver(t)
		defer func() {
			server.Stop()
			// Wait for server to fully stop and connections to close
			time.Sleep(200 * time.Millisecond)
		}()

		// Reset receiver to ensure clean state
		receiver.Reset()

		// Add timeout to prevent hanging
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, testBinaryPath, "metrics", "--metrics", "100", "--duration", "100ms", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure", "--batch=false")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with duration")
		assert.GreaterOrEqual(t, duration, 100*time.Millisecond, "Should run for at least the specified duration")
		assert.Less(t, duration, 5*time.Second, "Should not run much longer than specified duration")

		// Wait for all metrics to be processed
		time.Sleep(500 * time.Millisecond)
		metrics := receiver.GetMetrics()
		assert.NotEmpty(t, metrics, "Should have generated some metrics")
		assert.Less(t, len(metrics), 100, "Should not have generated 100 metrics due to duration limit")
	})

	t.Run("InfiniteDurationOverridesMetrics", func(t *testing.T) {
		server, endpoint, receiver := startMockReceiver(t)
		defer func() {
			server.Stop()
			// Wait for server to fully stop and connections to close
			time.Sleep(200 * time.Millisecond)
		}()

		// Reset receiver to ensure clean state
		receiver.Reset()

		cmd := exec.Command(testBinaryPath, "metrics", "--metrics", "50", "--duration", "inf", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure", "--batch=false")

		// Start the command first to ensure Process is available
		err := cmd.Start()
		require.NoError(t, err, "Failed to start command")

		// Now we can safely access cmd.Process
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-done:
			t.Log("Command completed - checking if this is expected")
		case <-time.After(3 * time.Second):
			if err := cmd.Process.Kill(); err != nil {
				t.Logf("Failed to kill process: %v", err)
			}
			t.Log("Command ran for 3 seconds as expected with infinite duration")
		}

		// Wait for all metrics to be processed
		time.Sleep(500 * time.Millisecond)
		metrics := receiver.GetMetrics()
		assert.NotEmpty(t, metrics, "Should have generated some metrics")
	})

	t.Run("BatchingWithBatchSize", func(t *testing.T) {
		server, endpoint, receiver := startMockReceiver(t)
		defer func() {
			server.Stop()
			// Wait for server to fully stop and connections to close
			time.Sleep(200 * time.Millisecond)
		}()

		// Reset receiver to ensure clean state
		receiver.Reset()

		// Add timeout to prevent hanging
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, testBinaryPath, "metrics", "--metrics", "6", "--workers", "1", "--batch-size", "3", "--rate", "1000", "--otlp-endpoint", endpoint, "--otlp-insecure")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with batching")
		assert.Less(t, duration, 6*time.Second, "Should complete quickly without connection issues")

		// Wait for all metrics to be processed
		time.Sleep(500 * time.Millisecond)
		metrics := receiver.GetMetrics()
		assert.Len(t, metrics, 2, "Should have received 2 batches (6 metrics / 3 per batch)")
	})

	t.Run("BatchingDisabled", func(t *testing.T) {
		server, endpoint, receiver := startMockReceiver(t)
		defer func() {
			server.Stop()
			// Wait for server to fully stop and connections to close
			time.Sleep(200 * time.Millisecond)
		}()

		// Reset receiver to ensure clean state
		receiver.Reset()

		// Add timeout to prevent hanging
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, testBinaryPath, "metrics", "--metrics", "4", "--workers", "1", "--rate", "1000", "--otlp-endpoint", endpoint, "--otlp-insecure", "--batch=false")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully without batching")
		assert.Less(t, duration, 6*time.Second, "Should complete quickly without connection issues")

		// Wait for all metrics to be processed
		time.Sleep(500 * time.Millisecond)
		metrics := receiver.GetMetrics()
		assert.Len(t, metrics, 4, "Should have received 4 individual metrics (no batching)")
	})

	t.Run("BatchingWithDefaultBatchSize", func(t *testing.T) {
		server, endpoint, receiver := startMockReceiver(t)
		defer func() {
			server.Stop()
			// Wait for server to fully stop and connections to close
			time.Sleep(200 * time.Millisecond)
		}()

		// Reset receiver to ensure clean state
		receiver.Reset()

		// Add timeout to prevent hanging
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, testBinaryPath, "metrics", "--metrics", "150", "--workers", "1", "--rate", "1000", "--otlp-endpoint", endpoint, "--otlp-insecure")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with default batching")
		assert.Less(t, duration, 12*time.Second, "Should complete within reasonable time")

		// Wait for all metrics to be processed
		time.Sleep(1 * time.Second)
		metrics := receiver.GetMetrics()
		// With default batch size 100, we should get 2 batches (100 + 50)
		assert.Len(t, metrics, 2, "Should have received 2 batches with default batch size 100")
	})
}
