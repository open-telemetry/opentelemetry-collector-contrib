// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drainprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/drainprocessor"

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

const storageKey = "drain_tree"

// getStorageClient resolves a storage.Client for the processor. Returns a
// NopClient when storageID is nil (storage not configured).
func getStorageClient(ctx context.Context, host component.Host, storageID *component.ID, componentID component.ID) (storage.Client, error) {
	if storageID == nil {
		return storage.NewNopClient(), nil
	}

	ext, ok := host.GetExtensions()[*storageID]
	if !ok {
		return nil, fmt.Errorf("storage extension %q not found", storageID)
	}

	storageExt, ok := ext.(storage.Extension)
	if !ok {
		return nil, fmt.Errorf("extension %q is not a storage extension", storageID)
	}

	// Normalize the component type to be immune to underscore renames.
	// See https://github.com/open-telemetry/opentelemetry-collector/issues/14988.
	normalizedType := strings.ReplaceAll(componentID.Type().String(), "_", "")
	normalizedID := component.MustNewIDWithName(normalizedType, componentID.Name())
	return storageExt.GetClient(ctx, component.KindProcessor, normalizedID, "")
}

// loadSnapshot attempts to restore tree state from storage. Returns true if a
// valid snapshot was loaded, false otherwise (caller should seed the tree).
func (p *drainProcessor) loadSnapshot(ctx context.Context) bool {
	data, err := p.storageClient.Get(ctx, storageKey)
	if err != nil {
		p.logger.Warn("failed to read snapshot from storage, starting fresh", zap.Error(err))
		return false
	}
	if len(data) == 0 {
		return false
	}

	if err := p.drain.Load(data); err != nil {
		p.logger.Warn("failed to load snapshot, starting fresh", zap.Error(err))
		return false
	}

	clusters := p.drain.ClusterCount()
	p.logger.Info("loaded drain tree snapshot from storage", zap.Int("clusters", clusters))

	if !p.warmedUp && clusters >= p.config.WarmupMinClusters {
		p.warmedUp = true
	}
	return true
}

// startPeriodicSave launches a background goroutine that saves the tree
// snapshot at the configured interval.
func (p *drainProcessor) startPeriodicSave() {
	ctx, cancel := context.WithCancel(context.Background())
	p.stopSave = cancel

	go func() {
		ticker := time.NewTicker(p.config.SaveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := p.saveSnapshot(ctx); err != nil {
					p.logger.Warn("periodic snapshot save failed", zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// saveSnapshot serializes the tree and writes it to storage. The write is
// skipped when the snapshot hash matches the last saved hash (no changes).
func (p *drainProcessor) saveSnapshot(ctx context.Context) error {
	p.mu.Lock()
	data, err := p.drain.Snapshot()
	p.mu.Unlock()
	if err != nil {
		return fmt.Errorf("snapshot serialization failed: %w", err)
	}

	h := fnv.New64a()
	h.Write(data)
	hash := h.Sum64()
	if hash == p.lastSnapshotHash {
		return nil
	}

	if err := p.storageClient.Set(ctx, storageKey, data); err != nil {
		return fmt.Errorf("failed to write snapshot to storage: %w", err)
	}
	p.lastSnapshotHash = hash
	p.logger.Debug("saved drain tree snapshot to storage", zap.Int("bytes", len(data)))
	return nil
}
