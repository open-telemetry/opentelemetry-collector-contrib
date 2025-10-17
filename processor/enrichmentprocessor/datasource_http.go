// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// HTTPDataSource implements DataSource for HTTP endpoints
// Supports both JSON and CSV data formats with automatic format detection
type HTTPDataSource struct {
	config     HTTPDataSourceConfig
	client     *http.Client
	store      *EnrichmentStore
	logger     *zap.Logger
	cancel     context.CancelFunc
	indexField []string
}

// NewHTTPDataSource creates a new HTTP data source with the given configuration
func NewHTTPDataSource(config HTTPDataSourceConfig, logger *zap.Logger, indexField []string) *HTTPDataSource {
	return &HTTPDataSource{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		store:      newEnrichmentStore(),
		logger:     logger,
		indexField: indexField,
	}
}

// Start starts the HTTP data source with initial load and periodic refresh
func (h *HTTPDataSource) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	h.cancel = cancel

	// Initial load
	if err := h.refresh(ctx); err != nil {
		return fmt.Errorf("failed to load initial data: %w", err)
	}

	// Start periodic refresh
	go func() {
		ticker := time.NewTicker(h.config.RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := h.refresh(ctx); err != nil {
					h.logger.Error("Failed to refresh HTTP data source", zap.Error(err))
				}
			}
		}
	}()

	return nil
}

// Stop stops the HTTP data source and cancels any ongoing operations
func (h *HTTPDataSource) Stop() error {
	if h.cancel != nil {
		h.cancel()
	}
	return nil
}

// Lookup performs a lookup for the given key using the underlying enrichment store
func (h *HTTPDataSource) Lookup(lookupField, key string) (enrichmentRow []string, index map[string]int, err error) {
	return h.store.Get(lookupField, key)
}

// refresh refreshes the HTTP data source by fetching and parsing new data
func (h *HTTPDataSource) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.config.URL, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add configured headers
	for key, value := range h.config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Determine format and parse data
	format := h.determineFormat(resp.Header.Get("Content-Type"))
	if format == "" {
		return fmt.Errorf("unable to determine data format from Content-Type header: %s. Please specify format explicitly in configuration", resp.Header.Get("Content-Type"))
	}

	var processedData [][]string
	var index map[string]int

	processedData, index, err = ParseData(body, format)
	if err != nil {
		return fmt.Errorf("failed to parse %s data: %w", format, err)
	}

	h.store.SetAll(processedData, index, h.indexField)
	h.logger.Info("Refreshed HTTP data source",
		zap.String("url", h.config.URL),
		zap.String("format", format),
		zap.Int("rows", len(processedData)))

	return nil
}

// determineFormat determines the data format from HTTP Content-Type header or config
func (h *HTTPDataSource) determineFormat(contentType string) string {
	// Use explicit format if specified in config
	if h.config.Format != "" {
		return h.config.Format
	}

	// Auto-detect from Content-Type header
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "text/csv") || strings.Contains(contentType, "application/csv") {
		return "csv"
	}
	if strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/json") {
		return "json"
	}

	return ""
}
