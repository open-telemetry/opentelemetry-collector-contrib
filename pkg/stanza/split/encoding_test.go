// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package split

import (
	"bufio"
	"bytes"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/unicode"
)

// TestUTF16LEFixedLengthRecords tests UTF-16LE encoding with fixed length records
// and no line termination (SAP audit log format).
// Regression test for https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/39011
func TestUTF16LEFixedLengthRecords(t *testing.T) {
	// Create test data: 10 SAP audit log records, 200 chars each, no newlines
	// The pattern from the issue: '([23])[A-Z][A-Z][A-Z0-9]\d{14}00'
	records := []string{
		"2AUK20250227000000002316500018D110.102.8BATCH_ALRI                      SAPMSSY1                                0501Z91_VALR_IF&&Z91_VAL_PLSTATUS                                   10.122.81.29        ",
		"2AUK20250227000001002316500018D110.102.8BATCH_ALRI                      SAPMSSY1                                0501Z91_VALR_IF&&Z91_VAL_PLSTATUS                                   10.122.81.30        ",
		"2AUK20250227000002002316500018D110.102.8BATCH_ALRI                      SAPMSSY1                                0501Z91_VALR_IF&&Z91_VAL_PLSTATUS                                   10.122.81.31        ",
		"3AUK20250227000003002316500018D110.102.8BATCH_ALRI                      SAPMSSY1                                0501Z91_VALR_IF&&Z91_VAL_PLSTATUS                                   10.122.81.32        ",
		"2AUK20250227000004002316500018D110.102.8BATCH_ALRI                      SAPMSSY1                                0501Z91_VALR_IF&&Z91_VAL_PLSTATUS                                   10.122.81.33        ",
		"2AUK20250227000005002316500018D110.102.8BATCH_ALRI                      SAPMSSY1                                0501Z91_VALR_IF&&Z91_VAL_PLSTATUS                                   10.122.81.34        ",
		"3AUK20250227000006002316500018D110.102.8BATCH_ALRI                      SAPMSSY1                                0501Z91_VALR_IF&&Z91_VAL_PLSTATUS                                   10.122.81.35        ",
		"2AUK20250227000007002316500018D110.102.8BATCH_ALRI                      SAPMSSY1                                0501Z91_VALR_IF&&Z91_VAL_PLSTATUS                                   10.122.81.36        ",
		"2AUK20250227000008002316500018D110.102.8BATCH_ALRI                      SAPMSSY1                                0501Z91_VALR_IF&&Z91_VAL_PLSTATUS                                   10.122.81.37        ",
		"2AUK20250227000009002316500018D110.102.8BATCH_ALRI                      SAPMSSY1                                0501Z91_VALR_IF&&Z91_VAL_PLSTATUS                                   10.122.81.38        ",
	}

	// Encode all records as UTF-16LE
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	encoder := utf16le.NewEncoder()

	var allData []byte
	for _, record := range records {
		encoded, err := encoder.Bytes([]byte(record))
		require.NoError(t, err)
		allData = append(allData, encoded...)
	}

	// Create the split function with the exact pattern from the issue
	cfg := Config{
		LineStartPattern: `([23])[A-Z][A-Z][A-Z0-9]\d{14}00`,
	}
	splitFunc, err := cfg.Func(utf16le, true, 0) // flushAtEOF=true
	require.NoError(t, err)

	// Use a scanner with the split function
	scanner := bufio.NewScanner(bytes.NewReader(allData))
	scanner.Split(splitFunc)

	// Count the tokens
	tokenCount := 0
	for scanner.Scan() {
		tokenCount++
		t.Logf("Token %d: %d bytes", tokenCount, len(scanner.Bytes()))
	}
	require.NoError(t, scanner.Err())

	// We should get 10 tokens (one per record)
	require.Equal(t, 10, tokenCount, "Expected 10 log records, got %d", tokenCount)
}

// TestUTF16LELineStartPatternDirect tests line_start_pattern with UTF-16LE encoding directly
func TestUTF16LELineStartPatternDirect(t *testing.T) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	encoder := utf16le.NewEncoder()

	// Simple test: 3 records with a simple pattern
	records := []string{
		"LOGSTART_01_content here___",
		"LOGSTART_02_more content___",
		"LOGSTART_03_final content__",
	}

	var allData []byte
	for _, record := range records {
		encoded, err := encoder.Bytes([]byte(record))
		require.NoError(t, err)
		allData = append(allData, encoded...)
	}

	// Test with line_start_pattern
	re := regexp.MustCompile(`(?m)LOGSTART_\d+_`)
	splitFunc := LineStartSplitFunc(re, false, true, utf16le, nil)

	scanner := bufio.NewScanner(bytes.NewReader(allData))
	scanner.Split(splitFunc)

	tokenCount := 0
	for scanner.Scan() {
		tokenCount++
		t.Logf("Token %d: %s", tokenCount, string(scanner.Bytes()))
	}
	require.NoError(t, scanner.Err())
	require.Equal(t, 3, tokenCount)
}

// TestUTF16LELineEndPatternDirect tests line_end_pattern with UTF-16LE encoding directly
func TestUTF16LELineEndPatternDirect(t *testing.T) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	encoder := utf16le.NewEncoder()

	// Simple test: 3 records with a line end pattern
	records := []string{
		"content here___LOGEND_01",
		"more content___LOGEND_02",
		"final content__LOGEND_03",
	}

	var allData []byte
	for _, record := range records {
		encoded, err := encoder.Bytes([]byte(record))
		require.NoError(t, err)
		allData = append(allData, encoded...)
	}

	// Test with line_end_pattern
	re := regexp.MustCompile(`(?m)LOGEND_\d+`)
	splitFunc := LineEndSplitFunc(re, false, true, utf16le, nil)

	scanner := bufio.NewScanner(bytes.NewReader(allData))
	scanner.Split(splitFunc)

	tokenCount := 0
	for scanner.Scan() {
		tokenCount++
		t.Logf("Token %d: %s", tokenCount, string(scanner.Bytes()))
	}
	require.NoError(t, scanner.Err())
	require.Equal(t, 3, tokenCount)
}
