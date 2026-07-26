// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

// getStorageClient resolves the storage.Client for the configured storage
// extension, or a no-op client if storageID is nil. Mirrors the pattern used
// by receiver/k8sobjectsreceiver and receiver/awscloudwatchreceiver.
func getStorageClient(ctx context.Context, host component.Host, storageID *component.ID, componentID component.ID) (storage.Client, error) {
	if storageID == nil {
		return storage.NewNopClient(), nil
	}

	ext, ok := host.GetExtensions()[*storageID]
	if !ok {
		return nil, fmt.Errorf("storage extension %q not found", storageID)
	}

	storageExtension, ok := ext.(storage.Extension)
	if !ok {
		return nil, fmt.Errorf("non-storage extension %q found", storageID)
	}

	// Make storage immune to component renames that add underscores to the component type.
	// This is a workaround for https://github.com/open-telemetry/opentelemetry-collector/issues/14988.
	normalizedComponentType := strings.ReplaceAll(componentID.Type().String(), "_", "")
	normalizedComponentID := component.MustNewIDWithName(normalizedComponentType, componentID.Name())
	return storageExtension.GetClient(ctx, component.KindReceiver, normalizedComponentID, "")
}

// collectionState is an in-memory cache of each collection's last-known rows,
// optionally write-through to a storage.Client for durability across
// restarts. The in-memory cache is what makes change-only diffing work when
// no storage extension is configured; the client only adds durability on top.
//
// All methods are nil-receiver-safe: a nil *collectionState behaves like an
// empty, unpersisted store (load returns nothing, save is a no-op), so
// osQueryReceiver values built without going through newOsQueryReceiver/start
// (as several existing tests do) still behave correctly.
type collectionState struct {
	mu     sync.RWMutex
	rows   map[string][]map[string]string
	client storage.Client
	logger *zap.Logger
}

func newCollectionState(client storage.Client, logger *zap.Logger) *collectionState {
	return &collectionState{
		rows:   make(map[string][]map[string]string),
		client: client,
		logger: logger,
	}
}

func (s *collectionState) load(ctx context.Context, name string) ([]map[string]string, error) {
	if s == nil {
		return nil, nil
	}

	s.mu.RLock()
	rows, ok := s.rows[name]
	s.mu.RUnlock()
	if ok {
		return rows, nil
	}

	if s.client == nil {
		return nil, nil
	}
	data, err := s.client.Get(ctx, name)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal persisted state for collection %q: %w", name, err)
	}

	s.mu.Lock()
	s.rows[name] = rows
	s.mu.Unlock()
	return rows, nil
}

func (s *collectionState) save(ctx context.Context, name string, rows []map[string]string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.rows[name] = rows
	s.mu.Unlock()

	if s.client == nil {
		return
	}
	data, err := json.Marshal(rows)
	if err != nil {
		s.logger.Error("Failed to marshal collection state", zap.String("collection", name), zap.Error(err))
		return
	}
	if err := s.client.Set(ctx, name, data); err != nil {
		s.logger.Error("Failed to persist collection state", zap.String("collection", name), zap.Error(err))
	}
}

func (s *collectionState) close(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close(ctx)
}
