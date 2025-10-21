// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"google.golang.org/api/googleapi"
)

func TestNewStorageExporter(t *testing.T) {
	tests := map[string]struct {
		getZone      func(context.Context) (string, error)
		getProjectID func(context.Context) (string, error)
		cfg          *Config
		expectsErr   string
	}{
		"region and project set": {
			cfg: &Config{
				Bucket: bucketConfig{
					ProjectID: "test",
					Region:    "test",
				},
			},
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
			expectsErr: "failed to determine project ID",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gcsExporter, err := newStorageExporter(t.Context(), test.cfg, test.getZone, test.getProjectID, zap.NewNop())
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
	mHost := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewID(encodingFailsID):    nil,
			component.MustNewID(encodingSucceedsID): &mockLogMarshaler{},
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
		require.ErrorContains(t, err, "is not a logs marshaler")
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
			component.MustNewID(encodingSucceedsID): &mockLogMarshaler{},
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

func TestConsumeLogs(t *testing.T) {
	uploadBucketName := "upload-bucket"
	newTestStorageEmulator(t, "", uploadBucketName)

	encodingSucceedsID := "id_success"
	mHost := &mockHost{
		extensions: map[component.ID]component.Component{
			component.MustNewID(encodingSucceedsID): &mockLogMarshaler{},
		},
	}
	id := component.MustNewID(encodingSucceedsID)
	gcsExporter := newTestGCSExporter(t, &Config{
		Bucket: bucketConfig{
			Name: uploadBucketName,
		},
		Encoding: &id,
	})

	errStart := gcsExporter.Start(t.Context(), mHost)
	require.NoError(t, errStart)

	err := gcsExporter.ConsumeLogs(t.Context(), plog.NewLogs())
	require.NoError(t, err)
}

func newTestGCSExporter(t *testing.T, cfg *Config) *storageExporter {
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
}

func (*mockLogMarshaler) MarshalLogs(_ plog.Logs) ([]byte, error) {
	return nil, nil
}

var _ plog.Marshaler = (*mockLogMarshaler)(nil)
