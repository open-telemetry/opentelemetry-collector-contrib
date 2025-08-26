// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

// FileDataSource implements DataSource for file-based sources
// Supports JSON and CSV formats with automatic file change detection
type FileDataSource struct {
	config     FileDataSourceConfig
	store      *EnrichmentStore
	logger     *zap.Logger
	cancel     context.CancelFunc
	lastMod    time.Time
	indexField []string
}

// NewFileDataSource creates a new file data source with the given configuration
func NewFileDataSource(config FileDataSourceConfig, logger *zap.Logger, indexField []string) *FileDataSource {
	return &FileDataSource{
		config:     config,
		store:      newEnrichmentStore(),
		logger:     logger,
		indexField: indexField,
	}
}

// Start starts the file data source with initial load and periodic refresh
func (f *FileDataSource) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	f.cancel = cancel

	// Initial load
	if err := f.refresh(ctx); err != nil {
		return fmt.Errorf("failed to load initial data: %w", err)
	}

	// Start periodic refresh
	go func() {
		ticker := time.NewTicker(f.config.RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := f.refresh(ctx); err != nil {
					f.logger.Error("Failed to refresh file data source", zap.Error(err))
				}
			}
		}
	}()

	return nil
}

// Stop stops the file data source and cancels any ongoing operations
func (f *FileDataSource) Stop() error {
	if f.cancel != nil {
		f.cancel()
	}
	return nil
}

// Lookup performs a lookup for the given key using the underlying enrichment store
func (f *FileDataSource) Lookup(lookupField, key string) (enrichmentRow []string, index map[string]int, err error) {
	return f.store.Get(lookupField, key)
}

// refresh loads data from the file if it has been modified since the last load
func (f *FileDataSource) refresh(_ context.Context) error {
	fileInfo, err := os.Stat(f.config.Path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if file has been modified
	if !f.lastMod.IsZero() && fileInfo.ModTime().Equal(f.lastMod) {
		return nil // No changes, skip refresh
	}

	file, err := os.Open(f.config.Path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileData, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse data based on configured format
	var data [][]string
	var index map[string]int

	data, index, err = ParseData(fileData, f.config.Format)
	if err != nil && strings.Contains(err.Error(), "unsupported format:") {
		err = fmt.Errorf("unsupported file format: %s", f.config.Format)
	}

	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	f.store.SetAll(data, index, f.indexField)
	f.lastMod = fileInfo.ModTime()

	f.logger.Info("Refreshed file data source",
		zap.String("path", f.config.Path),
		zap.String("format", f.config.Format),
		zap.Int("rows", len(data)))

	return nil
}
