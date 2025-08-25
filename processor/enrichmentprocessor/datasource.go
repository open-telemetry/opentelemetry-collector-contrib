// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package enrichmentprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/enrichmentprocessor"

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// DataSource represents an external data source for enrichment
type DataSource interface {
	// Lookup performs a lookup for the given key and returns enrichment data
	Lookup(lookupField, key string) (enrichmentRow []string, index map[string]int, err error)

	// Start starts the data source (e.g., periodic refresh)
	Start(ctx context.Context) error

	// Stop stops the data source
	Stop() error
}

// httpDataSource implements DataSource for HTTP endpoints
// Supports both JSON and CSV data formats.
// Format detection:
// 1. If config.Format is specified, uses that format
// 2. Otherwise, auto-detects based on Content-Type header:
//   - text/csv or application/csv -> CSV format
//   - anything else -> JSON format (default)
type httpDataSource struct {
	config     HTTPDataSourceConfig
	client     *http.Client
	lookup     *lookup
	logger     *zap.Logger
	cancel     context.CancelFunc
	indexField []string
}

// newHTTPDataSource creates a new HTTP data source
func newHTTPDataSource(config HTTPDataSourceConfig, logger *zap.Logger, indexField []string) *httpDataSource {
	return &httpDataSource{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		lookup:     newLookup(),
		logger:     logger,
		indexField: indexField,
	}
}

// Start starts the HTTP data source
func (h *httpDataSource) Start(ctx context.Context) error {
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
func (h *httpDataSource) Stop() error {
	if h.cancel != nil {
		h.cancel()
	}
	return nil
}

// Lookup performs a lookup for the given key
func (h *httpDataSource) Lookup(lookupField, key string) (enrichmentRow []string, index map[string]int, err error) {
	return h.lookup.Lookup(lookupField, key)
}

// refresh refreshes the HTTP data source
func (h *httpDataSource) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.config.URL, http.NoBody)
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
		return errors.New("HTTP request failed with status: " + strconv.Itoa(resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Determine content type and parse accordingly
	var processedData [][]string
	var index map[string]int

	// Use explicit format if specified in config, otherwise auto-detect from Content-Type
	format := h.config.Format
	if format == "" {
		format = detectFormatFromHeader(resp.Header.Get("Content-Type"))
	}
	if format == "" {
		return errors.New("unable to determine data format from Content-Type header: " + resp.Header.Get("Content-Type") + ". Please specify format explicitly in configuration")
	}

	switch strings.ToLower(format) {
	case "csv":
		// Parse as CSV
		processedData, index, err = parseCSV(body)
		if err != nil {
			return fmt.Errorf("failed to parse CSV: %w", err)
		}
	case "json":
		// Parse as JSON
		processedData, index, err = parseJSON(body)
		if err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	default:
		return errors.New("unsupported format: " + format)
	}

	h.lookup.SetAll(processedData, index, h.indexField)

	h.logger.Info("Refreshed HTTP data source", zap.String("url", h.config.URL))

	return nil
}

// detectFormatFromHeader detects the data format from HTTP Content-Type header
func detectFormatFromHeader(contentType string) string {
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "text/csv") || strings.Contains(contentType, "application/csv") {
		return "csv"
	}
	if strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/json") {
		return "json"
	}
	return ""
}

// parseJSON parses JSON data and creates a lookup map
// Used by both HTTP and File data sources
// Expected JSON input format: An array of objects where each object represents a row
// Example:
// [
//
//	{
//	  "service_name": "user-service",
//	  "owner_team": "platform-team",
//	  "environment": "production",
//	  "region": "us-east-1"
//	},
//	{
//	  "service_name": "payment-service",
//	  "owner_team": "payments-team",
//	  "environment": "production",
//	  "region": "us-west-2"
//	}
//
// ]
// This will be converted to a 2D string array with headers mapping to column indices
func parseJSON(data []byte) ([][]string, map[string]int, error) {
	var jsonData any
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, nil, err
	}

	// Only accept array of objects format
	dataArray, ok := jsonData.([]any)
	if !ok {
		return nil, nil, errors.New("JSON must be an array of objects")
	}

	if len(dataArray) == 0 {
		return nil, nil, errors.New("JSON array cannot be empty")
	}

	// First pass: collect all unique keys to build the header mapping
	allKeys := make(map[string]bool)
	for _, item := range dataArray {
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, nil, errors.New("all items in JSON array must be objects")
		}

		// Add all keys from this object
		for key := range itemMap {
			allKeys[key] = true
		}
	}

	// Convert set to index map
	index := make(map[string]int)
	i := 0
	for key := range allKeys {
		index[key] = i
		i++
	}

	// Second pass: process each row with all columns
	var rows [][]string
	for _, item := range dataArray {
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, nil, errors.New("all items in JSON array must be objects")
		}

		row := make([]string, len(index))
		for colName, colIndex := range index {
			if value, exists := itemMap[colName]; exists {
				row[colIndex] = fmt.Sprintf("%v", value)
			} else {
				row[colIndex] = "" // Empty string for missing values
			}
		}
		rows = append(rows, row)
	}

	return rows, index, nil
}

// parseCSV parses CSV data and creates a lookup map
// Used by both HTTP and File data sources
// Requires header row, all rows should have the same number of columns
func parseCSV(data []byte) ([][]string, map[string]int, error) {
	csvReader := csv.NewReader(strings.NewReader(string(data)))
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, nil, err
	}

	if len(records) < 2 {
		return nil, nil, errors.New("CSV file must have at least 2 rows (header + data)")
	}

	headers := records[0]
	index := make(map[string]int)

	// Build index map from headers
	for i, header := range headers {
		index[header] = i
	}

	// Data rows (excluding header)
	dataRows := records[1:]

	return dataRows, index, nil
}

// fileDataSource implements DataSource for file-based sources
type fileDataSource struct {
	config     FileDataSourceConfig
	lookup     *lookup
	logger     *zap.Logger
	cancel     context.CancelFunc
	lastMod    time.Time
	indexField []string
}

// newFileDataSource creates a new file data source
func newFileDataSource(config FileDataSourceConfig, logger *zap.Logger, indexField []string) *fileDataSource {
	return &fileDataSource{
		config:     config,
		lookup:     newLookup(),
		logger:     logger,
		indexField: indexField,
	}
}

// Start starts the file data source
func (f *fileDataSource) Start(ctx context.Context) error {
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
func (f *fileDataSource) Stop() error {
	if f.cancel != nil {
		f.cancel()
	}
	return nil
}

// Lookup performs a lookup for the given key
func (f *fileDataSource) Lookup(lookupField, key string) (enrichmentRow []string, index map[string]int, err error) {
	return f.lookup.Lookup(lookupField, key)
}

// refresh loads data from the file
func (f *fileDataSource) refresh(_ context.Context) error {
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

	var data [][]string
	var index map[string]int

	fileData, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	switch strings.ToLower(f.config.Format) {
	case "json":
		data, index, err = parseJSON(fileData)
	case "csv":
		data, index, err = parseCSV(fileData)
	default:
		return errors.New("unsupported file format: " + f.config.Format)
	}

	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	f.lookup.SetAll(data, index, f.indexField)
	f.lastMod = fileInfo.ModTime()

	f.logger.Info("Refreshed file data source", zap.String("path", f.config.Path))

	return nil
}
