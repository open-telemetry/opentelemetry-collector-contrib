// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedlogreceiver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedlogreceiver/internal/metadata"
)

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateWithInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		config    *Config
		expectErr bool
		errorMsg  string
	}{
		{
			name: "missing encoding",
			config: &Config{
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{
						Include: []string{"/var/db/diagnostics/Persist/*.tracev3"},
					},
				},
				Encoding: "",
			},
			expectErr: true,
			errorMsg:  "encoding must be macosunifiedlogencoding for macOS Unified Logging receiver",
		},
		{
			name: "invalid start_at",
			config: &Config{
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{
						Include: []string{"/var/db/diagnostics/Persist/*.tracev3"},
					},
					StartAt:            "invalid",
					FingerprintSize:    16,   // Set minimum valid fingerprint size
					MaxLogSize:         1024, // Set valid max log size
					MaxConcurrentFiles: 1,    // Set valid max concurrent files
				},
				Encoding: "macosunifiedlogencoding",
			},
			expectErr: true,
			errorMsg:  "invalid start_at location",
		},
		{
			name: "no include patterns",
			config: &Config{
				Config: fileconsumer.Config{
					Criteria: matcher.Criteria{
						Include: []string{},
					},
					FingerprintSize:    16,   // Set minimum valid fingerprint size
					MaxLogSize:         1024, // Set valid max log size
					MaxConcurrentFiles: 1,    // Set valid max concurrent files
				},
				Encoding: "macosunifiedlogencoding",
			},
			expectErr: true,
			errorMsg:  "'include' must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			sink := new(consumertest.LogsSink)

			// Create a mock host that provides the encoding extension
			host := &mockHost{
				extensions: map[component.ID]component.Component{
					component.MustNewID("macosunifiedlogencoding"): &mockEncodingExtension{},
				},
			}

			receiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), tt.config, sink)
			if tt.expectErr {
				if err != nil {
					assert.Contains(t, err.Error(), tt.errorMsg)
				} else {
					// Error might occur during Start instead of Create
					require.NotNil(t, receiver)
					err = receiver.Start(context.Background(), host)
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				if receiver != nil {
					require.NoError(t, receiver.Shutdown(context.Background()))
				}
			}
		})
	}
}

func TestCreateWithValidConfig(t *testing.T) {
	cfg := createTestConfig()

	// Create a mock host that can provide the encoding extension
	host := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewID("macosunifiedlogencoding"): &mockEncodingExtension{},
		},
	}

	receiver, err := NewFactory().CreateLogs(
		context.Background(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		new(consumertest.LogsSink),
	)
	require.NoError(t, err, "failed to create receiver with valid config")
	require.NotNil(t, receiver)

	// Test that we can start and stop the receiver
	err = receiver.Start(context.Background(), host)
	require.NoError(t, err, "failed to start receiver")

	err = receiver.Shutdown(context.Background())
	require.NoError(t, err, "failed to shutdown receiver")
}

func TestReceiveTraceV3Files(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for test files
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.tracev3")

	// Create a test traceV3 file (just binary data for testing)
	testData := []byte("mock tracev3 binary data for testing")
	err := os.WriteFile(testFile, testData, 0o600)
	require.NoError(t, err)

	// Create receiver configuration
	cfg := createTestConfig()
	cfg.Config.Include = []string{filepath.Join(tempDir, "*.tracev3")}
	cfg.Config.StartAt = "beginning"
	cfg.Config.PollInterval = 1 * time.Millisecond

	// Create receiver
	factory := NewFactory()
	sink := new(consumertest.LogsSink)

	// Create a mock host that provides the encoding extension
	host := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewID("macosunifiedlogencoding"): &mockEncodingExtension{},
		},
	}

	receiver, err := factory.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err, "failed to create receiver")

	// Start the receiver
	err = receiver.Start(context.Background(), host)
	require.NoError(t, err, "failed to start receiver")
	t.Logf("Receiver started successfully")

	// Wait for the file to be processed
	require.Eventually(t, expectNLogs(sink, 1), 2*time.Second, 10*time.Millisecond,
		"expected 1 log but got %d logs", sink.LogRecordCount())

	// Verify the log content
	logs := sink.AllLogs()
	require.Len(t, logs, 1)

	logRecord := logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Contains(t, logRecord.Body().AsString(), "Read traceV3 file")
	assert.Contains(t, logRecord.Body().AsString(), testFile)

	// Check attributes
	attrs := logRecord.Attributes()
	totalSize, exists := attrs.Get("file.total.size")
	assert.True(t, exists)
	assert.Equal(t, int64(len(testData)), totalSize.Int())

	tokenCount, exists := attrs.Get("file.token.count")
	assert.True(t, exists)
	assert.Equal(t, int64(1), tokenCount.Int())

	// Shutdown
	require.NoError(t, receiver.Shutdown(context.Background()))
}

func TestConsumeContract(t *testing.T) {
	t.Skip("Skipping consume contract test - requires extension setup")
	tmpDir := t.TempDir()
	filePattern := "test-*.tracev3"
	generator := &traceV3Generator{t: t, tmpDir: tmpDir, filePattern: filePattern}

	cfg := createTestConfig()
	cfg.RetryOnFailure.Enabled = true
	cfg.RetryOnFailure.InitialInterval = 1 * time.Millisecond
	cfg.RetryOnFailure.MaxInterval = 10 * time.Millisecond
	cfg.Config.Include = []string{filepath.Join(tmpDir, filePattern)}
	cfg.Config.StartAt = "beginning"

	receivertest.CheckConsumeContract(receivertest.CheckConsumeContractParams{
		T:             t,
		Factory:       NewFactory(),
		Signal:        pipeline.SignalLogs,
		Config:        cfg,
		Generator:     generator,
		GenerateCount: 100, // Lower count than filelog since we're creating files
	})
}

// Test helper functions

func createTestConfig() *Config {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.Encoding = "macosunifiedlogencoding"
	cfg.Config.Include = []string{"/tmp/test/*.tracev3"}
	cfg.Config.StartAt = "beginning"
	cfg.Config.PollInterval = 100 * time.Millisecond
	return cfg
}

func expectNLogs(sink *consumertest.LogsSink, expected int) func() bool {
	return func() bool { return sink.LogRecordCount() == expected }
}

// Mock types for testing

type mockHost struct {
	extensions map[component.ID]component.Component
}

func (h *mockHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

type mockEncodingExtension struct{}

func (e *mockEncodingExtension) Start(context.Context, component.Host) error { return nil }
func (e *mockEncodingExtension) Shutdown(context.Context) error              { return nil }

func (e *mockEncodingExtension) UnmarshalLogs(data []byte) (plog.Logs, error) {
	// Create a simple mock log for testing
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	logRecord.Body().SetStr(fmt.Sprintf("Mock decoded log from %d bytes", len(data)))
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return logs, nil
}

// traceV3Generator generates mock traceV3 files for testing
type traceV3Generator struct {
	t           *testing.T
	tmpDir      string
	filePattern string
	sequenceNum int64
}

func (g *traceV3Generator) Start() {
	// No setup needed for this generator
}

func (g *traceV3Generator) Stop() {
	// Clean up any files we created
	files, err := filepath.Glob(filepath.Join(g.tmpDir, g.filePattern))
	if err != nil {
		return
	}
	for _, file := range files {
		os.Remove(file)
	}
}

func (g *traceV3Generator) Generate() []receivertest.UniqueIDAttrVal {
	id := receivertest.UniqueIDAttrVal(strconv.FormatInt(atomic.AddInt64(&g.sequenceNum, 1), 10))

	// Create a unique filename for each generation
	fileName := fmt.Sprintf("test-%s.tracev3", id)
	filePath := filepath.Join(g.tmpDir, fileName)

	// Write mock binary data (in reality this would be actual traceV3 data)
	mockData := fmt.Sprintf("mock-tracev3-data-%s", id)
	err := os.WriteFile(filePath, []byte(mockData), 0o600)
	require.NoError(g.t, err)

	return []receivertest.UniqueIDAttrVal{id}
}
