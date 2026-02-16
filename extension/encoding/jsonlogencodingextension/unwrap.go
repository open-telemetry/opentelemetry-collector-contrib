// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jsonlogencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension"

import (
	"bufio"
	"bytes"
	"fmt"

	"github.com/goccy/go-json"
)

// extractRecords extracts individual JSON records from the input bytes using
// the provided JSONPath expression. It first tries to apply the JSONPath to
// the input as a single JSON document. If the input is not valid JSON (e.g.,
// NDJSON), it falls back to splitting by newlines.
func extractRecords(input []byte, jsonPathExpr string) ([][]byte, error) {
	input = bytes.TrimSpace(input)
	if len(input) == 0 {
		return nil, nil
	}

	// Try applying JSONPath to the input as a single JSON document
	if input[0] == '{' || input[0] == '[' {
		path, err := json.CreatePath(jsonPathExpr)
		if err != nil {
			return nil, fmt.Errorf("invalid JSONPath expression %q: %w", jsonPathExpr, err)
		}

		records, err := path.Extract(input)
		if err == nil {
			// JSONPath extraction succeeded
			result := make([][]byte, 0, len(records))
			for _, r := range records {
				trimmed := bytes.TrimSpace(r)
				if len(trimmed) > 0 {
					result = append(result, trimmed)
				}
			}
			return result, nil
		}

		// If the input starts with '{' and JSONPath extraction failed,
		// it may be NDJSON (multiple JSON objects separated by newlines)
		if input[0] == '{' {
			return splitNDJSON(input)
		}

		// For array-like input that failed JSONPath, return the error
		return nil, fmt.Errorf("failed to extract records using JSONPath %q: %w", jsonPathExpr, err)
	}

	return nil, fmt.Errorf("unsupported input format: expected JSON object or array, got %q", string(input[:min(len(input), 20)]))
}

// splitNDJSON splits newline-delimited JSON input into individual records.
func splitNDJSON(input []byte) ([][]byte, error) {
	var records [][]byte
	scanner := bufio.NewScanner(bytes.NewReader(input))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		// Make a copy since scanner reuses the buffer
		record := make([]byte, len(line))
		copy(record, line)
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading NDJSON input: %w", err)
	}
	return records, nil
}
