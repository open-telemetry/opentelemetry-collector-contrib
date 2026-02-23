// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/lestrrat-go/strftime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"google.golang.org/api/googleapi"
)

func TestNewStorageExporter(t *testing.T) {
	tests := map[string]struct {
		getZone      func(context.Context) (string, error)
		getProjectID func(context.Context) (string, error)
		cfg          *Config
		signal       signalType
		expectsErr   string
	}{
		"region and project set": {
			cfg: &Config{
				Bucket: bucketConfig{
					ProjectID: "test",
					Region:    "test",
				},
			},
			signal: signalTypeLogs,
		},
		"region missing and provider works": {
			getZone: func(_ context.Context) (string, error) {
				return "test", nil
			},
			cfg: &Config{
				Bucket: bucketConfig{
					ProjectID: "test",
				},
			},
			signal: signalTypeLogs,
		},
		"region missing and provider fails": {
			getZone: func(_ context.Context) (string, error) {
				return "", errors.New("not running on cloud")
			},
			cfg: &Config{
				Bucket: bucketConfig{
					ProjectID: "test",
				},
			},
			signal:     signalTypeLogs,
			expectsErr: "failed to determine region",
		},
		"project ID missing and provider works": {
			getProjectID: func(_ context.Context) (string, error) {
				return "test", nil
			},
			cfg: &Config{
				Bucket: bucketConfig{
					Region: "test",
				},
			},
			signal: signalTypeLogs,
		},
		"project ID missing and provider fails": {
			getProjectID: func(_ context.Context) (string, error) {
				return "", errors.New("not running on cloud")
			},
			cfg: &Config{
				Bucket: bucketConfig{
					Region: "test",
				},
			},
			signal:     signalTypeLogs,
			expectsErr: "failed to determine project ID",
		},
		"partition format valid and provider works": {
			cfg: &Config{
				Bucket: bucketConfig{
					ProjectID: "test",
					Region:    "test",
					Partition: partitionConfig{
						Format: "year=%Y",
					},
				},
			},
			signal: signalTypeLogs,
		},
		"partition format invalid and provider fails": {
			cfg: &Config{
				Bucket: bucketConfig{
					ProjectID: "test",
					Region:    "test",
					Partition: partitionConfig{
						Format: "year=%invalid",
					},
				},
			},
			signal:     signalTypeLogs,
			expectsErr: "failed to parse partition format",
		},
		"invalid signal type": {
			cfg: &Config{
				Bucket: bucketConfig{
					ProjectID: "test",
					Region:    "test",
				},
			},
			signal:     signalType("invalid"),
			expectsErr: "signal type \"invalid\" not recognized",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			signal := test.signal
			if signal == "" {
				signal = signalTypeLogs // default
			}

			gcsExporter, err := newStorageExporter(t.Context(), test.cfg, test.getZone, test.getProjectID, zap.NewNop(), signal)
			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, gcsExporter)
		})
	}
}

func TestStart(t *testing.T) {
	newBucketName := "new-bucket"
	bucketExistsName := "bucket-exists"

	newTestStorageEmulator(t, bucketExistsName, "")

	encodingSucceedsID := "id_success"
	encodingFailsID := "id_fail"
	encodingLogsOnlyID := "id_logs_only"
	mHost := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewID(encodingFailsID):    nil,
			component.MustNewID(encodingSucceedsID): &mockBothMarshaler{},
			component.MustNewID(encodingLogsOnlyID): &mockLogMarshaler{},
		},
	}

	id := component.MustNewID("unset")
	gcsExporter := newTestGCSExporter(t, &Config{
		Bucket: bucketConfig{
			Name: newBucketName,
		},
	})

	t.Run("unset encoding", func(t *testing.T) {
		err := gcsExporter.Start(t.Context(), mHost)
		require.NoError(t, err)
		require.Equal(t, &plog.JSONMarshaler{}, gcsExporter.logsMarshaler)
		require.Equal(t, &ptrace.JSONMarshaler{}, gcsExporter.tracesMarshaler)
	})

	gcsExporter.cfg.Encoding = &id
	t.Run("encoding id not present", func(t *testing.T) {
		err := gcsExporter.Start(t.Context(), mHost)
		require.ErrorContains(t, err, "unknown extension")
	})

	id = component.MustNewID(encodingFailsID)

	gcsExporter.cfg.Encoding = &id
	t.Run("encoding id not a logs marshaler", func(t *testing.T) {
		err := gcsExporter.Start(t.Context(), mHost)
		require.ErrorIs(t, err, errNotLogsMarshaler)
	})

	id = component.MustNewID(encodingLogsOnlyID)
	gcsExporter.cfg.Encoding = &id
	t.Run("encoding id only logs marshaler", func(t *testing.T) {
		err := gcsExporter.Start(t.Context(), mHost)
		require.NoError(t, err)
		require.IsType(t, &mockLogMarshaler{}, gcsExporter.logsMarshaler)
		// Traces marshaler remains default JSON since this is a logs exporter
		require.Equal(t, &ptrace.JSONMarshaler{}, gcsExporter.tracesMarshaler)
	})

	id = component.MustNewID(encodingSucceedsID)
	gcsExporter.cfg.Encoding = &id

	t.Run("create new bucket", func(t *testing.T) {
		err := gcsExporter.Start(t.Context(), mHost)
		require.NoError(t, err)
	})

	t.Run("bucket exists and cannot be reused", func(t *testing.T) {
		gcsExporter.cfg.Bucket.Name = bucketExistsName
		err := gcsExporter.Start(t.Context(), mHost)
		require.ErrorContains(t, err, "failed to create storage bucket")
	})

	t.Run("bucket exists and can be reused", func(t *testing.T) {
		gcsExporter.cfg.Bucket.Name = bucketExistsName
		gcsExporter.cfg.Bucket.ReuseIfExists = true
		err := gcsExporter.Start(t.Context(), mHost)
		require.NoError(t, err)
	})
}

func TestUploadFile(t *testing.T) {
	bucketExistsName := "bucket-exists"
	uploadBucketName := "upload-bucket"
	newTestStorageEmulator(t, bucketExistsName, uploadBucketName)

	encodingSucceedsID := "id_success"
	mHost := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewID(encodingSucceedsID): &mockBothMarshaler{},
		},
	}
	id := component.MustNewID(encodingSucceedsID)
	gcsExporter := newTestGCSExporter(t, &Config{
		Bucket: bucketConfig{
			Name: uploadBucketName,
		},
		Encoding: &id,
	})

	t.Run("empty content", func(t *testing.T) {
		err := gcsExporter.uploadFile(t.Context(), []byte{})
		require.NoError(t, err)
	})

	t.Run("upload content", func(t *testing.T) {
		errStart := gcsExporter.Start(t.Context(), mHost)
		require.NoError(t, errStart)
		err := gcsExporter.uploadFile(t.Context(), []byte("test content"))
		require.NoError(t, err)
	})
}

func TestGenerateFilename(t *testing.T) {
	// Fixed time for testing: 2023-10-25 14:30:00 UTC
	fixedTime := time.Date(2023, 10, 25, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name             string
		partitionFormat  string
		partitionPrefix  string
		filePrefix       string
		uniqueID         string
		compression      configcompression.Type
		expectedFilename string
	}{
		{
			name:             "empty partition format and prefix",
			uniqueID:         "uuid",
			compression:      "",
			expectedFilename: "uuid",
		},
		{
			name:             "file prefix set",
			filePrefix:       "logs",
			uniqueID:         "uuid",
			compression:      "",
			expectedFilename: "logs_uuid",
		},
		{
			name:             "file prefix with slash",
			filePrefix:       "folder/",
			uniqueID:         "uuid",
			compression:      "",
			expectedFilename: "folder/uuid",
		},
		{
			name:             "partition prefix set",
			partitionPrefix:  "my-logs",
			uniqueID:         "uuid",
			compression:      "",
			expectedFilename: "my-logs/uuid",
		},
		{
			name:             "partition format set",
			partitionFormat:  "year=%Y",
			uniqueID:         "uuid",
			compression:      "",
			expectedFilename: "year=2023/uuid",
		},
		{
			name:             "partition format and file prefix set",
			partitionFormat:  "year=%Y",
			filePrefix:       "logs",
			uniqueID:         "uuid",
			compression:      "",
			expectedFilename: "year=2023/logs_uuid",
		},
		{
			name:             "partition format, partition prefix and file prefix set",
			partitionFormat:  "year=%Y",
			partitionPrefix:  "archive",
			filePrefix:       "logs",
			uniqueID:         "uuid",
			compression:      "",
			expectedFilename: "archive/year=2023/logs_uuid",
		},
		{
			name:             "gzip compression",
			uniqueID:         "uuid",
			compression:      configcompression.TypeGzip,
			expectedFilename: "uuid.gz",
		},
		{
			name:             "zstd compression",
			uniqueID:         "uuid",
			compression:      configcompression.TypeZstd,
			expectedFilename: "uuid.zst",
		},
		{
			name:             "gzip compression with file prefix",
			filePrefix:       "logs",
			uniqueID:         "uuid",
			compression:      configcompression.TypeGzip,
			expectedFilename: "logs_uuid.gz",
		},
		{
			name:             "compression, partitioning and prefix combined",
			partitionFormat:  "year=%Y",
			partitionPrefix:  "archive",
			filePrefix:       "logs",
			uniqueID:         "uuid",
			compression:      configcompression.TypeGzip,
			expectedFilename: "archive/year=2023/logs_uuid.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var partitionFormat *strftime.Strftime
			if tt.partitionFormat != "" {
				var err error
				partitionFormat, err = strftime.New(tt.partitionFormat)
				require.NoError(t, err)
			}
			got := generateFilename(tt.uniqueID, tt.filePrefix, tt.partitionPrefix, tt.compression, partitionFormat, fixedTime)
			require.Equal(t, tt.expectedFilename, got)
		})
	}
}

func TestShutdown(t *testing.T) {
	gcsExporter := newTestGCSExporter(t, &Config{})
	err := gcsExporter.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestCapabilities(t *testing.T) {
	gcsExporter := newTestGCSExporter(t, &Config{})
	require.False(t, gcsExporter.Capabilities().MutatesData)
}

func TestConsumeLogs(t *testing.T) {
	uploadBucketName := "upload-bucket"
	newTestStorageEmulator(t, "", uploadBucketName)

	encodingSucceedsID := "id_success"
	encodingFailsID := "id_fail"
	encodingLogsOnlyID := "id_logs_only"
	mHost := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewID(encodingSucceedsID): &mockBothMarshaler{},
			component.MustNewID(encodingFailsID):    &mockLogMarshaler{shouldFail: true},
			component.MustNewID(encodingLogsOnlyID): &mockLogMarshaler{},
		},
	}

	tests := []struct {
		name              string
		id                string
		signal            signalType
		expectsConsumeErr string
		expectedMarshaler any
	}{
		{
			name:              "logs encoding fails",
			id:                encodingFailsID,
			signal:            signalTypeLogs,
			expectsConsumeErr: "failed to marshal logs",
		},
		{
			name:   "logs encoding succeeds",
			id:     encodingSucceedsID,
			signal: signalTypeLogs,
		},
		{
			name:              "traces with logs-only encoding falls back to JSON",
			id:                encodingLogsOnlyID,
			signal:            signalTypeTraces,
			expectedMarshaler: &ptrace.JSONMarshaler{},
		},
		{
			name:   "traces encoding succeeds",
			id:     encodingSucceedsID,
			signal: signalTypeTraces,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compID := component.MustNewID(tt.id)

			gcsExporter := newTestGCSExporter(t, &Config{
				Bucket: bucketConfig{
					Name: uploadBucketName,
				},
				Encoding: &compID,
			}, tt.signal)

			errStart := gcsExporter.Start(t.Context(), mHost)
			require.NoError(t, errStart)

			if tt.expectedMarshaler != nil {
				if tt.signal == signalTypeTraces {
					require.IsType(t, tt.expectedMarshaler, gcsExporter.tracesMarshaler)
				} else {
					require.IsType(t, tt.expectedMarshaler, gcsExporter.logsMarshaler)
				}
			}

			var err error
			if tt.signal == signalTypeTraces {
				err = gcsExporter.ConsumeTraces(t.Context(), ptrace.NewTraces())
			} else {
				err = gcsExporter.ConsumeLogs(t.Context(), plog.NewLogs())
			}

			if tt.expectsConsumeErr != "" {
				require.ErrorContains(t, err, tt.expectsConsumeErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCompression(t *testing.T) {
	// Test compression logic directly (independent of compression type)
	testData := []byte(strings.Repeat("This is a test string that will compress very well when repeated many times. ", 100))

	// Test with gzip compression
	gzipExporter := newTestGCSExporter(t, &Config{
		Bucket: bucketConfig{
			Compression: configcompression.TypeGzip,
		},
	})
	_, err := gzipExporter.compressContent(testData)
	require.NoError(t, err)

	// Test with zstd compression
	zstdExporter := newTestGCSExporter(t, &Config{
		Bucket: bucketConfig{
			Compression: configcompression.TypeZstd,
		},
	})
	_, err = zstdExporter.compressContent(testData)
	require.NoError(t, err)

	// Test with no compression
	noCompressionExporter := newTestGCSExporter(t, &Config{
		Bucket: bucketConfig{
			Compression: "",
		},
	})
	_, err = noCompressionExporter.compressContent(testData)
	require.NoError(t, err)

	tests := []struct {
		name            string
		compression     configcompression.Type
		expectExtension string
	}{
		{
			name:            "gzip compression",
			compression:     configcompression.TypeGzip,
			expectExtension: ".gz",
		},
		{
			name:            "zstd compression",
			compression:     configcompression.TypeZstd,
			expectExtension: ".zst",
		},
		{
			name:            "no compression",
			compression:     "",
			expectExtension: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test type-specific compression behavior
			exporter := newTestGCSExporter(t, &Config{
				Bucket: bucketConfig{
					Compression: tt.compression,
				},
			})

			compressedData, err := exporter.compressContent(testData)
			require.NoError(t, err)

			switch tt.compression {
			case "":
				// For uncompressed content, verify it matches the original directly
				assert.Equal(t, testData, compressedData, "Uncompressed content should match original")
			case configcompression.TypeGzip, configcompression.TypeZstd:
				// For compressed content, verify it can be decompressed back to original
				decompressed, closer := decompressData(t, compressedData, tt.compression)
				if closer != nil {
					defer closer()
				}

				// The decompressed content should match the original
				assert.Equal(t, testData, decompressed, "Decompressed content should match original")

				// Compressed content should be smaller than original
				assert.Less(t, len(compressedData), len(testData),
					"Compressed content should be smaller than uncompressed")
			}

			// Test filename generation includes correct extension
			filename := generateFilename("test-id", "test-prefix", "", tt.compression, nil, time.Now())
			assert.True(t, strings.HasSuffix(filename, tt.expectExtension),
				"Generated filename should end with %q, got %q", tt.expectExtension, filename)
		})
	}
}

// decompressData decompresses data based on the compression type and returns the decompressed data
// along with a closer function that should be deferred (or nil if no special closing needed)
func decompressData(t *testing.T, compressedData []byte, compression configcompression.Type) ([]byte, func()) {
	switch compression {
	case configcompression.TypeGzip:
		reader, err := gzip.NewReader(bytes.NewReader(compressedData))
		require.NoError(t, err)
		decompressed, err := io.ReadAll(reader)
		require.NoError(t, err)
		return decompressed, func() {
			closeErr := reader.Close()
			require.NoError(t, closeErr)
		}
	case configcompression.TypeZstd:
		reader, err := zstd.NewReader(bytes.NewReader(compressedData))
		require.NoError(t, err)
		decompressed, err := io.ReadAll(reader)
		require.NoError(t, err)
		return decompressed, func() {
			reader.Close()
		}
	default:
		t.Fatalf("Unsupported compression type: %s", compression)
		return nil, nil
	}
}

func newTestGCSExporter(t *testing.T, cfg *Config, signal ...signalType) *storageExporter {
	sig := signalTypeLogs
	if len(signal) > 0 {
		sig = signal[0]
	}
	exp, err := newStorageExporter(
		t.Context(),
		cfg,
		func(_ context.Context) (string, error) {
			return "test", nil
		},
		func(_ context.Context) (string, error) {
			return "test", nil
		},
		zap.NewNop(),
		sig,
	)
	require.NoError(t, err)
	return exp
}

func newTestStorageEmulator(t *testing.T, bucketExistsName, uploadBucketName string) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /storage/v1/b":
			// Handle bucket creation
			var body struct {
				Name string `json:"name"`
			}
			errDecode := json.NewDecoder(r.Body).Decode(&body)
			assert.NoError(t, errDecode)

			if body.Name == bucketExistsName {
				// Simulate bucket already exists
				w.WriteHeader(http.StatusConflict)
				errEncode := json.NewEncoder(w).Encode(googleapi.Error{Code: http.StatusConflict})
				assert.NoError(t, errEncode)
				return
			}
			w.WriteHeader(http.StatusOK)
			errEncode := json.NewEncoder(w).Encode(body)
			assert.NoError(t, errEncode)
		case "POST /upload/storage/v1/b/" + uploadBucketName + "/o":
			w.WriteHeader(http.StatusOK)
			errEncode := json.NewEncoder(w).Encode(map[string]any{
				"bucket": uploadBucketName,
			})
			assert.NoError(t, errEncode)
		default:
			t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(func() {
		server.Close()
	})

	t.Setenv("STORAGE_EMULATOR_HOST", server.Listener.Addr().String())
}

type mockHost struct {
	extensions map[component.ID]component.Component
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	return m.extensions
}

type mockLogMarshaler struct {
	extension.Extension
	shouldFail bool
}

func (m *mockLogMarshaler) MarshalLogs(_ plog.Logs) ([]byte, error) {
	if m.shouldFail {
		return nil, errors.New("marshaling failed")
	}
	return nil, nil
}

var _ plog.Marshaler = (*mockLogMarshaler)(nil)

type mockTraceMarshaler struct {
	extension.Extension
}

func (*mockTraceMarshaler) MarshalTraces(_ ptrace.Traces) ([]byte, error) {
	return nil, nil
}

var _ ptrace.Marshaler = (*mockTraceMarshaler)(nil)

type mockBothMarshaler struct {
	extension.Extension
}

func (*mockBothMarshaler) MarshalLogs(_ plog.Logs) ([]byte, error) {
	return nil, nil
}

func (*mockBothMarshaler) MarshalTraces(_ ptrace.Traces) ([]byte, error) {
	return nil, nil
}

var (
	_ plog.Marshaler   = (*mockBothMarshaler)(nil)
	_ ptrace.Marshaler = (*mockBothMarshaler)(nil)
)
