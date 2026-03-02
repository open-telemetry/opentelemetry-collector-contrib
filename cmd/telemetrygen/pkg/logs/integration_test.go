// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package logs

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
	"go.opentelemetry.io/collector/pdata/plog"
	collectorlogs "go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"google.golang.org/grpc"
)

// mockLogsReceiver is a mock gRPC receiver for testing
type mockLogsReceiver struct {
	collectorlogs.UnimplementedGRPCServer
	mu   sync.RWMutex
	logs []plog.Logs
}

func (m *mockLogsReceiver) Export(_ context.Context, req collectorlogs.ExportRequest) (collectorlogs.ExportResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, req.Logs())
	// Create a properly initialized response using the factory method
	return collectorlogs.NewExportResponse(), nil
}

func (m *mockLogsReceiver) GetLogs() []plog.Logs {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.logs
}

func (m *mockLogsReceiver) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = nil
}

// startMockReceiver starts a mock gRPC receiver on a local TCP port
func startMockReceiver(t *testing.T) (*grpc.Server, string, *mockLogsReceiver) {
	// Find an available port
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err, "Failed to create listener")

	port := lis.Addr().(*net.TCPAddr).Port
	lis.Close() // Close the listener to free the port

	// Create a new listener on the same port
	lis, err = net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	require.NoError(t, err, "Failed to create listener on port %d", port)

	server := grpc.NewServer()
	receiver := &mockLogsReceiver{}
	collectorlogs.RegisterGRPCServer(server, receiver)

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

	t.Run("RespectsLogsParameterWithBatching", func(t *testing.T) {
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
		cmd := exec.CommandContext(ctx, testBinaryPath, "logs", "--logs", "5", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure", "--batch", "--batch-size", "2")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with batching enabled")
		assert.Less(t, duration, 6*time.Second, "Should complete quickly without connection issues")

		// Wait for all logs to be processed
		time.Sleep(500 * time.Millisecond)
		logs := receiver.GetLogs()

		assert.Len(t, logs, 3, "Should have received exactly 3 log batches with batching enabled")
	})

	t.Run("RespectsLogsParameterWithoutBatching", func(t *testing.T) {
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
		cmd := exec.CommandContext(ctx, testBinaryPath, "logs", "--logs", "4", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure", "--batch=false")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with batching disabled")
		assert.Less(t, duration, 6*time.Second, "Should complete quickly without connection issues")

		// Wait for all logs to be processed
		time.Sleep(500 * time.Millisecond)
		logs := receiver.GetLogs()

		assert.Len(t, logs, 4, "Should have received exactly 4 log batches with batching disabled")
	})

	t.Run("DurationOverridesLogs", func(t *testing.T) {
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
		cmd := exec.CommandContext(ctx, testBinaryPath, "logs", "--logs", "100", "--duration", "100ms", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure", "--batch=false")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with duration")
		assert.GreaterOrEqual(t, duration, 100*time.Millisecond, "Should run for at least the specified duration")
		assert.Less(t, duration, 5*time.Second, "Should not run much longer than specified duration")

		// Wait for all logs to be processed
		time.Sleep(500 * time.Millisecond)
		logs := receiver.GetLogs()
		assert.NotEmpty(t, logs, "Should have generated some logs")
		assert.Less(t, len(logs), 100, "Should not have generated 100 logs due to duration limit")
	})

	t.Run("InfiniteDurationOverridesLogs", func(t *testing.T) {
		server, endpoint, receiver := startMockReceiver(t)
		defer func() {
			server.Stop()
			// Wait for server to fully stop and connections to close
			time.Sleep(200 * time.Millisecond)
		}()

		// Reset receiver to ensure clean state
		receiver.Reset()

		cmd := exec.Command(testBinaryPath, "logs", "--logs", "50", "--duration", "inf", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure", "--batch=false")

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

		// Wait for all logs to be processed
		time.Sleep(500 * time.Millisecond)
		logs := receiver.GetLogs()
		assert.NotEmpty(t, logs, "Should have generated some logs")
	})

	t.Run("BatchingWithBatchSize", func(t *testing.T) {
		server, endpoint, receiver := startMockReceiver(t)
		defer func() {
			server.Stop()
			time.Sleep(200 * time.Millisecond)
		}()

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, testBinaryPath, "logs", "--logs", "5", "--batch-size", "2", "--rate", "1000", "--otlp-endpoint", endpoint, "--otlp-insecure")
		err := cmd.Run()
		require.NoError(t, err, "Failed to run telemetrygen")

		time.Sleep(1 * time.Second)

		logs := receiver.GetLogs()
		assert.Len(t, logs, 3, "Expected 3 batches with batch size 2")
		assert.Equal(t, 2, logs[0].LogRecordCount(), "First batch should have 2 logs")
		assert.Equal(t, 2, logs[1].LogRecordCount(), "Second batch should have 2 logs")
		assert.Equal(t, 1, logs[2].LogRecordCount(), "Third batch should have 1 log")
	})

	t.Run("NoBatchingWhenFlagDisabled", func(t *testing.T) {
		server, endpoint, receiver := startMockReceiver(t)
		defer func() {
			server.Stop()
			time.Sleep(200 * time.Millisecond)
		}()

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, testBinaryPath, "logs", "--logs", "3", "--rate", "1000", "--otlp-endpoint", endpoint, "--otlp-insecure", "--batch=false")
		err := cmd.Run()
		require.NoError(t, err, "Failed to run telemetrygen")

		time.Sleep(1 * time.Second)

		logs := receiver.GetLogs()
		assert.Len(t, logs, 3, "Expected 3 individual logs when batching is disabled")
		assert.Equal(t, 1, logs[0].LogRecordCount(), "Each log should be sent individually")
		assert.Equal(t, 1, logs[1].LogRecordCount(), "Each log should be sent individually")
		assert.Equal(t, 1, logs[2].LogRecordCount(), "Each log should be sent individually")
	})

	t.Run("RespectsLogsParameterWithBatching", func(t *testing.T) {
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
		cmd := exec.CommandContext(ctx, testBinaryPath, "logs", "--logs", "5", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure", "--batch", "--batch-size", "2")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with batching enabled")
		assert.Less(t, duration, 6*time.Second, "Should complete quickly without connection issues")

		// Wait for all logs to be processed
		time.Sleep(500 * time.Millisecond)
		logs := receiver.GetLogs()

		assert.Len(t, logs, 3, "Should have received exactly 3 log batches with batching enabled")
	})

	t.Run("RespectsLogsParameterWithoutBatching", func(t *testing.T) {
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
		cmd := exec.CommandContext(ctx, testBinaryPath, "logs", "--logs", "4", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure", "--batch=false")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with batching disabled")
		assert.Less(t, duration, 6*time.Second, "Should complete quickly without connection issues")

		// Wait for all logs to be processed
		time.Sleep(500 * time.Millisecond)
		logs := receiver.GetLogs()

		assert.Len(t, logs, 4, "Should have received exactly 4 log batches with batching disabled")
	})
}
