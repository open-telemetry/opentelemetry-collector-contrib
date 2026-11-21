// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/metadata"
)

// s3Upload is one PutObject request received by the fake S3 endpoint.
type s3Upload struct {
	bucket string
	key    string
	body   string
}

// newFakeS3 starts a path-style S3 endpoint that records every PutObject.
func newFakeS3(t *testing.T) (*httptest.Server, func() []s3Upload) {
	t.Helper()
	var mu sync.Mutex
	var uploads []s3Upload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			bucket, key, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
			mu.Lock()
			uploads = append(uploads, s3Upload{bucket: bucket, key: key, body: string(body)})
			mu.Unlock()
			w.Header().Set("ETag", `"test"`)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server, func() []s3Upload {
		mu.Lock()
		defer mu.Unlock()
		return append([]s3Upload(nil), uploads...)
	}
}

func newBatchTestConfig(endpoint string) *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.S3Uploader.Endpoint = endpoint
	cfg.S3Uploader.S3Bucket = "default-bucket"
	cfg.S3Uploader.S3ForcePathStyle = true
	cfg.S3Uploader.DisableSSL = true
	cfg.S3Uploader.AccessKey = "access"
	cfg.S3Uploader.SecretKey = "secret"
	cfg.S3Uploader.S3PartitionFormat = ""
	cfg.MarshalerName = Body
	return cfg
}

func enableQueueBatching(cfg *Config) {
	// Batches are only flushed on shutdown, so every request that reaches the
	// queue can be merged with the others.
	cfg.QueueSettings.Get().Batch = configoptional.Some(exporterhelper.BatchConfig{
		FlushTimeout: time.Minute,
		Sizer:        exporterhelper.RequestSizerTypeItems,
		MinSize:      1000,
		Partition:    exporterhelper.NewDefaultPartitionConfig(),
	})
}

// logsWithResources returns one resource per entry, each with a single log
// record whose body is the entry's value.
func logsWithResources(attrKey string, values ...string) plog.Logs {
	ld := plog.NewLogs()
	for _, v := range values {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr(attrKey, v)
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-from-" + v)
	}
	return ld
}

func exportLogs(t *testing.T, cfg *Config, ld plog.Logs) {
	t.Helper()
	exp, err := NewFactory().CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, exp.ConsumeLogs(t.Context(), ld))
	// Shutdown flushes the queue and any pending batch.
	require.NoError(t, exp.Shutdown(t.Context()))
}

func TestResourceAttrsToS3PrefixWithQueueBatching(t *testing.T) {
	server, uploads := newFakeS3(t)
	cfg := newBatchTestConfig(server.URL)
	cfg.ResourceAttrsToS3.S3Prefix = "s3.prefix"
	enableQueueBatching(cfg)

	exportLogs(t, cfg, logsWithResources("s3.prefix", "prefix-a", "prefix-b"))

	got := uploads()
	require.Len(t, got, 2, "each prefix must be uploaded as its own object")
	for _, u := range got {
		prefix, _, _ := strings.Cut(u.key, "/")
		other := map[string]string{"prefix-a": "prefix-b", "prefix-b": "prefix-a"}[prefix]
		require.NotEmpty(t, other, "unexpected key %q", u.key)
		assert.Contains(t, u.body, "log-from-"+prefix, "key %q", u.key)
		assert.NotContains(t, u.body, "log-from-"+other, "key %q holds another prefix's logs", u.key)
	}
}

func TestResourceAttrsToS3BucketWithMixedResources(t *testing.T) {
	server, uploads := newFakeS3(t)
	cfg := newBatchTestConfig(server.URL)
	cfg.ResourceAttrsToS3.S3Bucket = "s3.bucket"

	exportLogs(t, cfg, logsWithResources("s3.bucket", "bucket-a", "bucket-b"))

	got := uploads()
	require.Len(t, got, 2, "each bucket must receive its own object")
	for _, u := range got {
		assert.Contains(t, u.body, "log-from-"+u.bucket, "bucket %q", u.bucket)
	}
}

func TestResourceAttrsToS3PrefixWithoutQueueBatching(t *testing.T) {
	server, uploads := newFakeS3(t)
	cfg := newBatchTestConfig(server.URL)
	cfg.ResourceAttrsToS3.S3Prefix = "s3.prefix"

	exportLogs(t, cfg, logsWithResources("s3.prefix", "prefix-a", "prefix-b"))

	got := uploads()
	require.Len(t, got, 2)
	for _, u := range got {
		prefix, _, _ := strings.Cut(u.key, "/")
		assert.Contains(t, u.body, "log-from-"+prefix, "key %q", u.key)
	}
}

func TestS3QueueSettingsPartitionKeys(t *testing.T) {
	t.Run("no routing attributes leaves the queue unchanged", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		enableQueueBatching(cfg)
		qs := s3QueueSettings(cfg)
		assert.Empty(t, qs.Get().Batch.Get().Partition.MetadataKeys)
	})

	t.Run("batching disabled leaves the queue unchanged", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.ResourceAttrsToS3.S3Prefix = "s3.prefix"
		qs := s3QueueSettings(cfg)
		assert.False(t, qs.Get().Batch.HasValue())
	})

	t.Run("routing attributes are added once, keeping user keys", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.ResourceAttrsToS3.S3Bucket = "s3.bucket"
		cfg.ResourceAttrsToS3.S3Prefix = "s3.prefix"
		enableQueueBatching(cfg)
		cfg.QueueSettings.Get().Batch.Get().Partition.MetadataKeys = []string{"tenant", "S3.Prefix"}

		qs := s3QueueSettings(cfg)
		got := qs.Get().Batch.Get().Partition.MetadataKeys
		assert.Equal(t, []string{"tenant", "S3.Prefix", "s3.bucket"}, got)
		// The user's config is not modified.
		assert.Equal(t, []string{"tenant", "S3.Prefix"}, cfg.QueueSettings.Get().Batch.Get().Partition.MetadataKeys)
	})
}
