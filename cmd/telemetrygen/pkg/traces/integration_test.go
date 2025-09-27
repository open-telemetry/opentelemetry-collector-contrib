// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package traces

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
	"go.opentelemetry.io/collector/pdata/ptrace"
	collectortraces "go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
)

// mockTracesReceiver is a mock gRPC receiver for testing
type mockTracesReceiver struct {
	collectortraces.UnimplementedGRPCServer
	mu     sync.RWMutex
	traces []ptrace.Traces
}

func (m *mockTracesReceiver) Export(_ context.Context, req collectortraces.ExportRequest) (collectortraces.ExportResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.traces = append(m.traces, req.Traces())
	// Create a properly initialized response using the factory method
	return collectortraces.NewExportResponse(), nil
}

func (m *mockTracesReceiver) GetTraces() []ptrace.Traces {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.traces
}

func (m *mockTracesReceiver) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.traces = nil
}

// startMockReceiver starts a mock gRPC receiver on a local TCP port
func startMockReceiver(t *testing.T) (*grpc.Server, string, *mockTracesReceiver) {
	// Find an available port
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err, "Failed to create listener")

	port := lis.Addr().(*net.TCPAddr).Port
	lis.Close() // Close the listener to free the port

	// Create a new listener on the same port
	lis, err = net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	require.NoError(t, err, "Failed to create listener on port %d", port)

	server := grpc.NewServer()
	receiver := &mockTracesReceiver{}
	collectortraces.RegisterGRPCServer(server, receiver)

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
	binaryName := fmt.Sprintf("telemetrygen-test-%d", time.Now().UnixNano())
	buildCmd := exec.Command("go", "build", "-o", binaryName, ".")
	buildCmd.Dir = buildDir
	err := buildCmd.Run()
	require.NoError(t, err, "Failed to build telemetrygen")

	testBinaryPath := fmt.Sprintf("../../../telemetrygen/%s", binaryName)
	defer os.Remove(testBinaryPath)

	t.Run("RespectsTracesParameter", func(t *testing.T) {
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
		cmd := exec.CommandContext(ctx, testBinaryPath, "traces", "--traces", "3", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with --traces parameter")
		assert.Less(t, duration, 8*time.Second, "Should complete quickly without connection issues")

		// Wait for all traces to be processed
		time.Sleep(500 * time.Millisecond)
		traces := receiver.GetTraces()

		// Count unique trace IDs across all received traces
		traceIDs := make(map[string]bool)
		for _, trace := range traces {
			for i := 0; i < trace.ResourceSpans().Len(); i++ {
				resourceSpan := trace.ResourceSpans().At(i)
				for j := 0; j < resourceSpan.ScopeSpans().Len(); j++ {
					scopeSpan := resourceSpan.ScopeSpans().At(j)
					for k := 0; k < scopeSpan.Spans().Len(); k++ {
						span := scopeSpan.Spans().At(k)
						traceIDs[span.TraceID().String()] = true
					}
				}
			}
		}

		assert.Len(t, traceIDs, 3, "Should have received exactly 3 unique traces")
	})

	t.Run("DurationOverridesTraces", func(t *testing.T) {
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
		cmd := exec.CommandContext(ctx, testBinaryPath, "traces", "--traces", "100", "--duration", "100ms", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure")

		start := time.Now()
		err := cmd.Run()
		duration := time.Since(start)

		assert.NoError(t, err, "telemetrygen should complete successfully with duration")
		assert.GreaterOrEqual(t, duration, 100*time.Millisecond, "Should run for at least the specified duration")
		assert.Less(t, duration, 5*time.Second, "Should not run much longer than specified duration")

		// Wait for all traces to be processed
		time.Sleep(500 * time.Millisecond)
		traces := receiver.GetTraces()
		assert.NotEmpty(t, traces, "Should have generated some traces")
		assert.Less(t, len(traces), 100, "Should not have generated 100 traces due to duration limit")
	})

	t.Run("InfiniteDurationOverridesTraces", func(t *testing.T) {
		server, endpoint, receiver := startMockReceiver(t)
		defer func() {
			server.Stop()
			// Wait for server to fully stop and connections to close
			time.Sleep(200 * time.Millisecond)
		}()

		// Reset receiver to ensure clean state
		receiver.Reset()

		cmd := exec.Command(testBinaryPath, "traces", "--traces", "50", "--duration", "inf", "--workers", "1", "--otlp-endpoint", endpoint, "--otlp-insecure")

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

		// Wait for all traces to be processed
		time.Sleep(500 * time.Millisecond)
		traces := receiver.GetTraces()
		assert.NotEmpty(t, traces, "Should have generated some traces")
	})
}
