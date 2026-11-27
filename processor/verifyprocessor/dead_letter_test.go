// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

type memStorage struct {
	mu     sync.Mutex
	items  map[string][]byte
	setErr error
	closed bool
}

func newMemStorage() *memStorage {
	return &memStorage{items: map[string][]byte{}}
}

func (m *memStorage) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.items[key]
	if !ok {
		return nil, nil
	}
	return append([]byte(nil), value...), nil
}

func (m *memStorage) Set(_ context.Context, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setErr != nil {
		return m.setErr
	}
	m.items[key] = append([]byte(nil), value...)
	return nil
}

func (m *memStorage) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
	return nil
}

func (m *memStorage) Batch(ctx context.Context, ops ...*storage.Operation) error {
	for _, op := range ops {
		switch op.Type {
		case storage.Get:
			value, err := m.Get(ctx, op.Key)
			if err != nil {
				return err
			}
			op.Value = value
		case storage.Set:
			if err := m.Set(ctx, op.Key, op.Value); err != nil {
				return err
			}
		case storage.Delete:
			if err := m.Delete(ctx, op.Key); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown storage operation %d", op.Type)
		}
	}
	return nil
}

func (m *memStorage) Close(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *memStorage) values() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]byte, 0, len(m.items))
	for _, value := range m.items {
		out = append(out, append([]byte(nil), value...))
	}
	return out
}

func (m *memStorage) keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.items))
	for key := range m.items {
		out = append(out, key)
	}
	return out
}

func dlqProc(t *testing.T, mode string, dl DeadLetterConfig, store storage.Client) *verifyProcessor {
	t.Helper()
	dl.Enabled = true
	if dl.KeyPrefix == "" {
		dl.KeyPrefix = "dlq/"
	}
	if dl.MaxEntrySizeBytes == 0 {
		dl.MaxEntrySizeBytes = defaultMaxEntrySizeBytes
	}
	p := newVerifyProc(t, &Config{
		FailureMode:         mode,
		VerificationProfile: "default",
		DeadLetter:          dl,
	}, &staticProvider{hmacKey: []byte("test-hmac")})
	p.storageClient = store
	return p
}

func failedLogs(id, body string) plog.Logs {
	attrs := map[string]string{}
	if id != "" {
		attrs[attrRecordID] = id
	}
	return makeLogs(body, attrs, algorithmHMACSHA256, "", "")
}

func TestDeadLetterWritesFailure(t *testing.T) {
	store := newMemStorage()
	p := dlqProc(t, failureModeMark, DeadLetterConfig{}, store)
	sink := &consumertest.LogsSink{}
	p.nextConsumer = sink

	ld := failedLogs("rec-1", "missing-body")
	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	require.Equal(t, 1, sink.LogRecordCount())

	values := store.values()
	require.Len(t, values, 1)
	var entry deadLetterEntry
	require.NoError(t, json.Unmarshal(values[0], &entry))
	assert.Equal(t, reasonMissingIntegrityValue, entry.Reason)
	assert.Equal(t, failureModeMark, entry.FailureMode)
	assert.Equal(t, "default", entry.Profile)
	assert.Equal(t, "rec-1", entry.LogID)
	assert.Equal(t, algorithmHMACSHA256, entry.Algorithm)
	assert.False(t, entry.Truncated)
	assert.Contains(t, string(entry.Logs), "missing-body")
	assert.NotContains(t, string(entry.Logs), "audit.integrity.algorithm")
}

func TestDeadLetterSkipsSuccessfulRecord(t *testing.T) {
	store := newMemStorage()
	p := dlqProc(t, failureModeStrict, DeadLetterConfig{}, store)
	hmacKey := []byte("test-hmac")
	ld := makeLogs("ok", nil, algorithmHMACSHA256, "", "")
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	payload, err := p.serializeLogRecord(lr)
	require.NoError(t, err)
	mac := hmac.New(sha256.New, hmacKey)
	_, err = mac.Write(payload)
	require.NoError(t, err)
	lr.Attributes().PutStr(attrIntegrityValue, base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	assert.Empty(t, store.values())
}

func TestDeadLetterFilters(t *testing.T) {
	t.Run("reason", func(t *testing.T) {
		store := newMemStorage()
		p := dlqProc(t, failureModeMark, DeadLetterConfig{Reasons: []string{reasonIntegrityMismatch}}, store)
		require.NoError(t, p.ConsumeLogs(t.Context(), failedLogs("rec-1", "body")))
		assert.Empty(t, store.values())
	})

	t.Run("failure mode", func(t *testing.T) {
		store := newMemStorage()
		p := dlqProc(t, failureModeMark, DeadLetterConfig{FailureModes: []string{failureModeDrop}}, store)
		require.NoError(t, p.ConsumeLogs(t.Context(), failedLogs("rec-1", "body")))
		assert.Empty(t, store.values())
	})

	t.Run("include resource", func(t *testing.T) {
		store := newMemStorage()
		includeRecord := false
		includeResource := true
		p := dlqProc(t, failureModeMark, DeadLetterConfig{
			IncludeRecord:   &includeRecord,
			IncludeResource: &includeResource,
		}, store)
		ld := failedLogs("rec-1", "secret-body")
		ld.ResourceLogs().At(0).Resource().Attributes().PutStr("tenant", "acme")
		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		values := store.values()
		require.Len(t, values, 1)
		assert.Contains(t, string(values[0]), "acme")
		assert.NotContains(t, string(values[0]), "secret-body")
	})

	t.Run("omit record", func(t *testing.T) {
		store := newMemStorage()
		include := false
		p := dlqProc(t, failureModeStrict, DeadLetterConfig{IncludeRecord: &include}, store)
		err := p.ConsumeLogs(t.Context(), failedLogs("rec-1", "secret-body"))
		require.Error(t, err)
		assert.True(t, consumererror.IsPermanent(err))
		values := store.values()
		require.Len(t, values, 1)
		assert.NotContains(t, string(values[0]), "secret-body")
		assert.NotContains(t, string(values[0]), `"logs"`)
	})
}

func TestDeadLetterTruncatesOversizedEntry(t *testing.T) {
	store := newMemStorage()
	p := dlqProc(t, failureModeMark, DeadLetterConfig{MaxEntrySizeBytes: 400}, store)
	body := strings.Repeat("x", 2048)
	require.NoError(t, p.ConsumeLogs(t.Context(), failedLogs("rec-1", body)))
	values := store.values()
	require.Len(t, values, 1)
	var entry deadLetterEntry
	require.NoError(t, json.Unmarshal(values[0], &entry))
	assert.True(t, entry.Truncated)
	assert.Empty(t, entry.Logs)
	assert.NotContains(t, string(values[0]), body)
}

func TestDeadLetterStorageErrors(t *testing.T) {
	setErr := errors.New("storage down")

	t.Run("continue", func(t *testing.T) {
		store := newMemStorage()
		store.setErr = setErr
		p := dlqProc(t, failureModeMark, DeadLetterConfig{}, store)
		sink := &consumertest.LogsSink{}
		p.nextConsumer = sink
		require.NoError(t, p.ConsumeLogs(t.Context(), failedLogs("rec-1", "body")))
		require.Equal(t, 1, sink.LogRecordCount())
		assert.Empty(t, store.values())
	})

	t.Run("continue logs once per batch", func(t *testing.T) {
		store := newMemStorage()
		store.setErr = setErr
		p := dlqProc(t, failureModeMark, DeadLetterConfig{}, store)
		core, logs := observer.New(zap.ErrorLevel)
		p.settings.Logger = zap.New(core)
		ld := failedLogs("rec-1", "one")
		records := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
		records.AppendEmpty().Body().SetStr("two")
		records.AppendEmpty().Body().SetStr("three")

		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		entries := logs.All()
		require.Len(t, entries, 1)
		fields := entries[0].ContextMap()
		assert.Equal(t, int64(3), fields["failed_entries"])
		assert.Equal(t, reasonMissingIntegrityValue, fields["first_reason"])
		assert.Equal(t, setErr.Error(), fields["error"])
	})

	t.Run("fail closed", func(t *testing.T) {
		store := newMemStorage()
		store.setErr = setErr
		failClosed := true
		p := dlqProc(t, failureModeMark, DeadLetterConfig{FailOnStorageError: &failClosed}, store)
		sink := &consumertest.LogsSink{}
		p.nextConsumer = sink
		err := p.ConsumeLogs(t.Context(), failedLogs("rec-1", "body"))
		require.ErrorIs(t, err, setErr)
		assert.Equal(t, 0, sink.LogRecordCount())
	})

	t.Run("fail closed leaves batch unstamped", func(t *testing.T) {
		store := newMemStorage()
		store.setErr = setErr
		failClosed := true
		p := dlqProc(t, failureModeMark, DeadLetterConfig{FailOnStorageError: &failClosed}, store)
		ld := failedLogs("rec-1", "body")
		err := p.ConsumeLogs(t.Context(), ld)
		require.ErrorIs(t, err, setErr)
		requireNoStatus(t, ld.ResourceLogs().At(0).Resource().Attributes())
		requireNoStatus(t, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes())
	})
}

func TestDeadLetterDropStillStoresRecord(t *testing.T) {
	store := newMemStorage()
	p := dlqProc(t, failureModeDrop, DeadLetterConfig{}, store)
	sink := &consumertest.LogsSink{}
	p.nextConsumer = sink

	err := p.ConsumeLogs(t.Context(), failedLogs("rec-1", "dropped-body"))
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
	assert.Equal(t, 0, sink.LogRecordCount())
	values := store.values()
	require.Len(t, values, 1)
	assert.Contains(t, string(values[0]), "dropped-body")
	assert.Contains(t, string(values[0]), failureModeDrop)
}

func tooLargeBatch() (plog.Logs, string) {
	longID := strings.Repeat("i", 2048)
	ld := failedLogs(longID, "big-body")
	second := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
	second.Body().SetStr("small-body")
	second.Attributes().PutStr(attrRecordID, "small-id")
	return ld, longID
}

func TestDeadLetterTooLargeContinuesBatch(t *testing.T) {
	t.Run("fail closed", func(t *testing.T) {
		ld, longID := tooLargeBatch()
		store := newMemStorage()
		failClosed := true
		p := dlqProc(t, failureModeMark, DeadLetterConfig{
			MaxEntrySizeBytes:  minEntrySizeBytes,
			FailOnStorageError: &failClosed,
		}, store)
		sink := &consumertest.LogsSink{}
		p.nextConsumer = sink

		err := p.ConsumeLogs(t.Context(), ld)
		require.ErrorIs(t, err, errDeadLetterTooLarge)
		assert.True(t, consumererror.IsPermanent(err))
		assert.Equal(t, 0, sink.LogRecordCount())
		requireNoStatus(t, ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Attributes())

		values := store.values()
		require.Len(t, values, 1)
		assert.Contains(t, string(values[0]), "small-id")
		assert.NotContains(t, string(values[0]), longID)
	})

	t.Run("fail open", func(t *testing.T) {
		ld, _ := tooLargeBatch()
		store := newMemStorage()
		p := dlqProc(t, failureModeMark, DeadLetterConfig{MaxEntrySizeBytes: minEntrySizeBytes}, store)
		sink := &consumertest.LogsSink{}
		p.nextConsumer = sink

		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		require.Equal(t, 2, sink.LogRecordCount())
		values := store.values()
		require.Len(t, values, 1)
		assert.Contains(t, string(values[0]), "small-id")
	})
}

func TestDeadLetterKeyIncludesInstance(t *testing.T) {
	store := newMemStorage()
	p := dlqProc(t, failureModeMark, DeadLetterConfig{}, store)
	ld := failedLogs("rec-1", "one")
	second := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().AppendEmpty()
	second.Body().SetStr("two")
	second.Attributes().PutStr(attrRecordID, "rec-2")

	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	keys := store.keys()
	require.Len(t, keys, 2)
	assert.NotEqual(t, keys[0], keys[1])
	_, err := hex.DecodeString(p.dlqInstance)
	require.NoError(t, err)
	assert.Len(t, p.dlqInstance, 16)
	for _, key := range keys {
		assert.True(t, strings.HasPrefix(key, "dlq/"+p.dlqInstance+"-"), key)
	}
}

func TestDeadLetterFileStorageNamespaces(t *testing.T) {
	t.Run("one processor shared by two pipelines", func(t *testing.T) {
		storageID, host, dir := newFileStorage(t)
		proc := startDeadLetterProc(t, component.MustNewID("verify"), storageID, host)
		require.NoError(t, proc.ConsumeLogs(t.Context(), failedLogs("rec-a", "pipeline-a")))
		require.NoError(t, proc.ConsumeLogs(t.Context(), failedLogs("rec-b", "pipeline-b")))

		bodies := deadLetterBodies(t, proc.storageClient)
		require.Len(t, bodies, 2)
		joined := strings.Join(bodies, "\n")
		assert.Contains(t, joined, "pipeline-a")
		assert.Contains(t, joined, "pipeline-b")
		assert.Equal(t, []string{"processor_verify__dead_letter"}, storageFiles(t, dir))
	})

	t.Run("two component ids", func(t *testing.T) {
		storageID, host, dir := newFileStorage(t)
		a := startDeadLetterProc(t, component.MustNewIDWithName("verify", "a"), storageID, host)
		b := startDeadLetterProc(t, component.MustNewIDWithName("verify", "b"), storageID, host)
		require.NoError(t, a.ConsumeLogs(t.Context(), failedLogs("rec-a", "body-a")))
		require.NoError(t, b.ConsumeLogs(t.Context(), failedLogs("rec-b", "body-b")))

		aBodies := deadLetterBodies(t, a.storageClient)
		bBodies := deadLetterBodies(t, b.storageClient)
		require.Len(t, aBodies, 1)
		require.Len(t, bBodies, 1)
		assert.Contains(t, aBodies[0], "body-a")
		assert.NotContains(t, aBodies[0], "body-b")
		assert.Contains(t, bBodies[0], "body-b")
		assert.NotContains(t, bBodies[0], "body-a")
		assert.ElementsMatch(t, []string{
			"processor_verify_a_dead_letter",
			"processor_verify_b_dead_letter",
		}, storageFiles(t, dir))
	})

	t.Run("second open of the same component id fails", func(t *testing.T) {
		storageID, host, dir := newFileStorage(t)
		id := component.MustNewID("verify")
		first := startDeadLetterProc(t, id, storageID, host)
		require.NoError(t, first.ConsumeLogs(t.Context(), failedLogs("rec-a", "kept")))

		second, err := NewFactory().CreateLogs(t.Context(), deadLetterSettings(id), deadLetterFileConfig(storageID), consumertest.NewNop())
		require.NoError(t, err)
		err = second.Start(t.Context(), host)
		require.Error(t, err)
		assert.ErrorContains(t, err, "dead letter storage")
		require.NoError(t, second.Shutdown(t.Context()))

		assert.Equal(t, []string{"processor_verify__dead_letter"}, storageFiles(t, dir))
		bodies := deadLetterBodies(t, first.storageClient)
		require.Len(t, bodies, 1)
		assert.Contains(t, bodies[0], "kept")
	})
}

func TestDeadLetterSameComponentInDistinctPipelines(t *testing.T) {
	storageID, host, dir := newFileStorage(t)
	id := component.MustNewID("verify")
	cfg := deadLetterFileConfig(storageID)
	require.NoError(t, cfg.Validate())

	procs := make([]*verifyProcessor, 0, 2)
	for _, name := range []string{"a", "b"} {
		settings := deadLetterSettings(id)
		settings.PipelineID = pipeline.NewIDWithName(pipeline.SignalLogs, name)
		proc, err := NewFactory().CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, proc.Shutdown(context.WithoutCancel(t.Context())))
		})
		require.NoError(t, proc.Start(t.Context(), host), "pipeline logs/%s should start", name)
		procs = append(procs, proc.(*verifyProcessor))
	}

	require.NoError(t, procs[0].ConsumeLogs(t.Context(), failedLogs("rec-a", "body-a")))
	require.NoError(t, procs[1].ConsumeLogs(t.Context(), failedLogs("rec-b", "body-b")))

	aBodies := deadLetterBodies(t, procs[0].storageClient)
	bBodies := deadLetterBodies(t, procs[1].storageClient)
	require.Len(t, aBodies, 1)
	require.Len(t, bBodies, 1)
	assert.Contains(t, aBodies[0], "body-a")
	assert.NotContains(t, aBodies[0], "body-b")
	assert.Contains(t, bBodies[0], "body-b")
	assert.NotContains(t, bBodies[0], "body-a")
	assert.ElementsMatch(t, []string{
		"processor_verify__dead_letter-logs~002Fa",
		"processor_verify__dead_letter-logs~002Fb",
	}, storageFiles(t, dir))
}

func TestDeadLetterFileStoragePersistsAcrossRestart(t *testing.T) {
	storageID, host, dir := newFileStorage(t)
	id := component.MustNewID("verify")
	first := startDeadLetterProc(t, id, storageID, host)
	require.NoError(t, first.ConsumeLogs(t.Context(), failedLogs("rec-1", "stored-body")))

	snapshot := map[string][]byte{}
	require.NoError(t, first.storageClient.(storage.Walker).Walk(t.Context(), func(key string, value []byte) ([]*storage.Operation, error) {
		snapshot[key] = append([]byte(nil), value...)
		return nil, nil
	}))
	require.Len(t, snapshot, 1)

	require.NoError(t, first.Shutdown(t.Context()))
	require.Nil(t, first.storageClient)

	second := startDeadLetterProc(t, id, storageID, host)
	reopened := map[string][]byte{}
	require.NoError(t, second.storageClient.(storage.Walker).Walk(t.Context(), func(key string, value []byte) ([]*storage.Operation, error) {
		reopened[key] = append([]byte(nil), value...)
		return nil, nil
	}))
	require.Equal(t, snapshot, reopened)
	for _, value := range reopened {
		assert.Contains(t, string(value), "rec-1")
		assert.Contains(t, string(value), "stored-body")
	}
	assert.Equal(t, []string{"processor_verify__dead_letter"}, storageFiles(t, dir))
}

func newFileStorage(t *testing.T) (component.ID, component.Host, string) {
	t.Helper()
	dir := t.TempDir()
	factory := filestorage.NewFactory()
	cfg := factory.CreateDefaultConfig().(*filestorage.Config)
	cfg.Directory = dir
	cfg.Timeout = 200 * time.Millisecond
	ext, err := factory.Create(t.Context(), extensiontest.NewNopSettings(factory.Type()), cfg)
	require.NoError(t, err)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, ext.Shutdown(context.Background()))
	})
	id := component.MustNewID("file_storage")
	return id, storagetest.NewStorageHost().WithExtension(id, ext), dir
}

func deadLetterFileConfig(storageID component.ID) *Config {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.FailureMode = failureModeMark
	cfg.VerificationProfile = "default"
	cfg.KeySource = KeySourceConfig{Type: keySourceFile, File: &FileKeyConfig{HMACKey: "testdata/dev_hmac_key.txt"}}
	cfg.DeadLetter.Enabled = true
	cfg.DeadLetter.StorageID = ptrID(storageID)
	return cfg
}

func deadLetterSettings(id component.ID) processor.Settings {
	settings := processortest.NewNopSettings(typ)
	settings.ID = id
	return settings
}

func startDeadLetterProc(t *testing.T, id, storageID component.ID, host component.Host) *verifyProcessor {
	t.Helper()
	cfg := deadLetterFileConfig(storageID)
	require.NoError(t, cfg.Validate())
	proc, err := NewFactory().CreateLogs(t.Context(), deadLetterSettings(id), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, proc.Start(t.Context(), host))
	t.Cleanup(func() {
		require.NoError(t, proc.Shutdown(context.Background()))
	})
	return proc.(*verifyProcessor)
}

func deadLetterBodies(t *testing.T, client storage.Client) []string {
	t.Helper()
	var bodies []string
	require.NoError(t, client.(storage.Walker).Walk(t.Context(), func(_ string, value []byte) ([]*storage.Operation, error) {
		bodies = append(bodies, string(value))
		return nil, nil
	}))
	return bodies
}

func storageFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names
}

func TestDeadLetterRequiresClient(t *testing.T) {
	failClosed := true
	p := dlqProc(t, failureModeStrict, DeadLetterConfig{FailOnStorageError: &failClosed}, nil)
	err := p.ConsumeLogs(t.Context(), failedLogs("rec-1", "body"))
	require.ErrorIs(t, err, errDeadLetterClient)
}

func TestDeadLetterStartShutdown(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.FailureMode = failureModeMark
	cfg.VerificationProfile = "default"
	cfg.KeySource = KeySourceConfig{Type: keySourceFile, File: &FileKeyConfig{HMACKey: "testdata/dev_hmac_key.txt"}}
	cfg.DeadLetter.Enabled = true
	cfg.DeadLetter.StorageID = ptrID(storagetest.NewStorageID("dlq"))
	cfg.DeadLetter.KeyPrefix = "audit/"
	require.NoError(t, cfg.Validate())

	sink := &consumertest.LogsSink{}
	proc, err := factory.CreateLogs(t.Context(), processortest.NewNopSettings(typ), cfg, sink)
	require.NoError(t, err)

	require.Error(t, proc.Start(t.Context(), storagetest.NewStorageHost()))

	nonStorage := storagetest.NewStorageHost().WithNonStorageExtension("other")
	cfg.DeadLetter.StorageID = ptrID(storagetest.NewNonStorageID("other"))
	proc, err = factory.CreateLogs(t.Context(), processortest.NewNopSettings(typ), cfg, sink)
	require.NoError(t, err)
	require.Error(t, proc.Start(t.Context(), nonStorage))

	cfg.DeadLetter.StorageID = ptrID(storagetest.NewStorageID("dlq"))
	proc, err = factory.CreateLogs(t.Context(), processortest.NewNopSettings(typ), cfg, sink)
	require.NoError(t, err)
	host := storagetest.NewStorageHost().WithInMemoryStorageExtension("dlq")
	require.NoError(t, proc.Start(t.Context(), host))

	ld := failedLogs("rec-9", "stored-body")
	require.NoError(t, proc.ConsumeLogs(t.Context(), ld))
	require.Equal(t, 1, sink.LogRecordCount())

	vp := proc.(*verifyProcessor)
	var stored [][]byte
	require.NoError(t, vp.storageClient.(storage.Walker).Walk(t.Context(), func(key string, value []byte) ([]*storage.Operation, error) {
		if strings.HasPrefix(key, "audit/") {
			stored = append(stored, append([]byte(nil), value...))
		}
		return nil, nil
	}))
	require.Len(t, stored, 1)
	assert.Contains(t, string(stored[0]), "rec-9")
	assert.Contains(t, string(stored[0]), "stored-body")

	require.NoError(t, proc.Shutdown(t.Context()))
	require.Nil(t, vp.storageClient)
	require.NoError(t, proc.Shutdown(t.Context()))
}
