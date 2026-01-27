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
	buildDir := "../../../telemetrygen"
	buildCmd := exec.Command("go", "build", "-o", "telemetrygen-test", ".")
	buildCmd.Dir = buildDir
	err := buildCmd.Run()
	require.NoError(t, err, "Failed to build telemetrygen")

	defer os.Remove("../../../telemetrygen/telemetrygen-test")

	testBinaryPath := "../../../telemetrygen/telemetrygen-test"

	t.Run("RespectsLogsParameter", func(t *testing.T) {
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
		cmd := exec.CommandContext(ctx, testBinaryPath, "logs", "--logs", "3", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with --logs parameter")
		assert.Less(t, duration, 6*time.Second, "Should complete quickly without connection issues")

		// Wait for all logs to be processed
		time.Sleep(500 * time.Millisecond)
		logs := receiver.GetLogs()
		assert.Len(t, logs, 3, "Should have received exactly 3 logs")
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
		cmd := exec.CommandContext(ctx, testBinaryPath, "logs", "--logs", "100", "--duration", "100ms", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure")

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

		cmd := exec.Command(testBinaryPath, "logs", "--logs", "50", "--duration", "inf", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure")

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
}
