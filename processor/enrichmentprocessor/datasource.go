// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DataSource represents an external data source for enrichment
type DataSource interface {
	// Lookup performs a lookup for the given key and returns enrichment data
	Lookup(ctx context.Context, key string) (map[string]interface{}, error)

	// Start starts the data source (e.g., periodic refresh)
	Start(ctx context.Context) error

	// Stop stops the data source
	Stop() error
}

// HTTPDataSource implements DataSource for HTTP endpoints
type HTTPDataSource struct {
	config HTTPDataSourceConfig
	client *http.Client
	data   map[string]map[string]interface{}
	mutex  sync.RWMutex
	logger *zap.Logger
	cancel context.CancelFunc
}

// NewHTTPDataSource creates a new HTTP data source
func NewHTTPDataSource(config HTTPDataSourceConfig, logger *zap.Logger) *HTTPDataSource {
	return &HTTPDataSource{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		data:   make(map[string]map[string]interface{}),
		logger: logger,
	}
}

// Start starts the HTTP data source
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

// Stop stops the HTTP data source
func (h *HTTPDataSource) Stop() error {
	if h.cancel != nil {
		h.cancel()
	}
	return nil
}

// Lookup performs a lookup for the given key
func (h *HTTPDataSource) Lookup(ctx context.Context, key string) (map[string]interface{}, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	data, exists := h.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	return data, nil
}

// refresh fetches data from the HTTP endpoint
func (h *HTTPDataSource) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", h.config.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
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

	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Process data based on JSONPath if specified
	processedData, err := h.processJSONData(jsonData)
	if err != nil {
		return fmt.Errorf("failed to process JSON data: %w", err)
	}

	h.mutex.Lock()
	h.data = processedData
	h.mutex.Unlock()

	h.logger.Info("Refreshed HTTP data source", zap.String("url", h.config.URL), zap.Int("records", len(processedData)))

	return nil
}

// processJSONData processes the JSON data and creates a lookup map
func (h *HTTPDataSource) processJSONData(data interface{}) (map[string]map[string]interface{}, error) {
	result := make(map[string]map[string]interface{})

	// For simplicity, assume data is an array of objects
	if dataArray, ok := data.([]interface{}); ok {
		for _, item := range dataArray {
			if itemMap, ok := item.(map[string]interface{}); ok {
				// Try common key fields first
				var key string
				for _, keyField := range []string{"name", "id", "key", "service_name", "hostname"} {
					if value, exists := itemMap[keyField]; exists {
						if keyStr, ok := value.(string); ok {
							key = keyStr
							break
						}
					}
				}
				// If no common key field found, use the first string value
				if key == "" {
					for _, value := range itemMap {
						if keyStr, ok := value.(string); ok {
							key = keyStr
							break
						}
					}
				}
				if key != "" {
					result[key] = itemMap
				}
			}
		}
	} else if dataMap, ok := data.(map[string]interface{}); ok {
		// If it's a single object, use each key as lookup key
		for key, value := range dataMap {
			if valueMap, ok := value.(map[string]interface{}); ok {
				result[key] = valueMap
			}
		}
	}

	return result, nil
}

// FileDataSource implements DataSource for file-based sources
type FileDataSource struct {
	config  FileDataSourceConfig
	data    map[string]map[string]interface{}
	mutex   sync.RWMutex
	logger  *zap.Logger
	cancel  context.CancelFunc
	lastMod time.Time
}

// NewFileDataSource creates a new file data source
func NewFileDataSource(config FileDataSourceConfig, logger *zap.Logger) *FileDataSource {
	return &FileDataSource{
		config: config,
		data:   make(map[string]map[string]interface{}),
		logger: logger,
	}
}

// Start starts the file data source
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

// Stop stops the file data source
func (f *FileDataSource) Stop() error {
	if f.cancel != nil {
		f.cancel()
	}
	return nil
}

// Lookup performs a lookup for the given key
func (f *FileDataSource) Lookup(ctx context.Context, key string) (map[string]interface{}, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	data, exists := f.data[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	return data, nil
}

// refresh loads data from the file
func (f *FileDataSource) refresh(ctx context.Context) error {
	fileInfo, err := os.Stat(f.config.Path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if file has been modified
	if !f.lastMod.IsZero() && fileInfo.ModTime().Equal(f.lastMod) {
		return nil // No changes
	}

	file, err := os.Open(f.config.Path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var data map[string]map[string]interface{}

	switch strings.ToLower(f.config.Format) {
	case "json":
		data, err = f.parseJSON(file)
	case "csv":
		data, err = f.parseCSV(file)
	default:
		return fmt.Errorf("unsupported file format: %s", f.config.Format)
	}

	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	f.mutex.Lock()
	f.data = data
	f.lastMod = fileInfo.ModTime()
	f.mutex.Unlock()

	f.logger.Info("Refreshed file data source", zap.String("path", f.config.Path), zap.Int("records", len(data)))

	return nil
}

// parseJSON parses JSON file
func (f *FileDataSource) parseJSON(reader io.Reader) (map[string]map[string]interface{}, error) {
	var jsonData interface{}
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&jsonData); err != nil {
		return nil, err
	}

	result := make(map[string]map[string]interface{})

	if dataArray, ok := jsonData.([]interface{}); ok {
		for _, item := range dataArray {
			if itemMap, ok := item.(map[string]interface{}); ok {
				// Try common key fields first (same as HTTP data source)
				var key string
				for _, keyField := range []string{"name", "id", "key", "service_name", "hostname"} {
					if value, exists := itemMap[keyField]; exists {
						if keyStr, ok := value.(string); ok {
							key = keyStr
							break
						}
					}
				}
				// If no common key field found, use the first string value
				if key == "" {
					for _, value := range itemMap {
						if keyStr, ok := value.(string); ok {
							key = keyStr
							break
						}
					}
				}
				if key != "" {
					result[key] = itemMap
				}
			}
		}
	} else if dataMap, ok := jsonData.(map[string]interface{}); ok {
		for key, value := range dataMap {
			if valueMap, ok := value.(map[string]interface{}); ok {
				result[key] = valueMap
			}
		}
	}

	return result, nil
}

// parseCSV parses CSV file
func (f *FileDataSource) parseCSV(reader io.Reader) (map[string]map[string]interface{}, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1 // Allow variable number of fields
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file must have at least 2 rows (header + data)")
	}

	headers := records[0]
	result := make(map[string]map[string]interface{})

	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) != len(headers) {
			continue // Skip malformed rows
		}

		rowData := make(map[string]interface{})
		var key string

		for j, value := range record {
			if j < len(headers) {
				rowData[headers[j]] = value
				if j == 0 { // Use first column as key
					key = value
				}
			}
		}

		if key != "" {
			result[key] = rowData
		}
	}

	return result, nil
}
