// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package split

import (
	"bufio"
	"bytes"
	"regexp"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

// generateTestData creates test data with the specified number of records
func generateTestData(numRecords, recordSize int, encoder func([]byte) []byte) []byte {
	var data []byte
	for i := range numRecords {
		record := make([]byte, recordSize)
		// Create a record starting with "LOGSTART_XX_" where XX is the record number
		prefix := []byte("LOGSTART_")
		copy(record, prefix)
		record[9] = byte('0' + (i/10)%10)
		record[10] = byte('0' + i%10)
		record[11] = '_'
		// Fill the rest with 'x'
		for j := 12; j < recordSize; j++ {
			record[j] = 'x'
		}
		data = append(data, encoder(record)...)
	}
	return data
}

// utf8Encoder returns the input unchanged (UTF-8 is the default)
func utf8Encoder(data []byte) []byte {
	return data
}

// utf16LEEncoder encodes data as UTF-16LE
func utf16LEEncoder(data []byte) []byte {
	encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	result, _ := encoder.Bytes(data)
	return result
}

// BenchmarkLineStartSplitFunc_UTF8 benchmarks line_start_pattern with UTF-8 encoding
func BenchmarkLineStartSplitFunc_UTF8(b *testing.B) {
	benchmarkLineStartSplitFunc(b, unicode.UTF8, utf8Encoder, 100, 200)
}

// BenchmarkLineStartSplitFunc_UTF16LE benchmarks line_start_pattern with UTF-16LE encoding
func BenchmarkLineStartSplitFunc_UTF16LE(b *testing.B) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	benchmarkLineStartSplitFunc(b, utf16le, utf16LEEncoder, 100, 200)
}

// BenchmarkLineStartSplitFunc_UTF8_LargeRecords benchmarks with larger records
func BenchmarkLineStartSplitFunc_UTF8_LargeRecords(b *testing.B) {
	benchmarkLineStartSplitFunc(b, unicode.UTF8, utf8Encoder, 50, 1000)
}

// BenchmarkLineStartSplitFunc_UTF16LE_LargeRecords benchmarks with larger records
func BenchmarkLineStartSplitFunc_UTF16LE_LargeRecords(b *testing.B) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	benchmarkLineStartSplitFunc(b, utf16le, utf16LEEncoder, 50, 1000)
}

func benchmarkLineStartSplitFunc(b *testing.B, enc encoding.Encoding, encoder func([]byte) []byte, numRecords, recordSize int) {
	data := generateTestData(numRecords, recordSize, encoder)
	re := regexp.MustCompile(`(?m)LOGSTART_\d+_`)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		splitFunc := LineStartSplitFunc(re, false, true, enc, nil)
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Split(splitFunc)

		count := 0
		for scanner.Scan() {
			count++
		}
		if scanner.Err() != nil {
			b.Fatal(scanner.Err())
		}
		if count != numRecords {
			b.Fatalf("expected %d records, got %d", numRecords, count)
		}
	}
}

// BenchmarkLineEndSplitFunc_UTF8 benchmarks line_end_pattern with UTF-8 encoding
func BenchmarkLineEndSplitFunc_UTF8(b *testing.B) {
	benchmarkLineEndSplitFunc(b, unicode.UTF8, utf8Encoder, 100, 200)
}

// BenchmarkLineEndSplitFunc_UTF16LE benchmarks line_end_pattern with UTF-16LE encoding
func BenchmarkLineEndSplitFunc_UTF16LE(b *testing.B) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	benchmarkLineEndSplitFunc(b, utf16le, utf16LEEncoder, 100, 200)
}

// BenchmarkLineEndSplitFunc_UTF8_LargeRecords benchmarks with larger records
func BenchmarkLineEndSplitFunc_UTF8_LargeRecords(b *testing.B) {
	benchmarkLineEndSplitFunc(b, unicode.UTF8, utf8Encoder, 50, 1000)
}

// BenchmarkLineEndSplitFunc_UTF16LE_LargeRecords benchmarks with larger records
func BenchmarkLineEndSplitFunc_UTF16LE_LargeRecords(b *testing.B) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	benchmarkLineEndSplitFunc(b, utf16le, utf16LEEncoder, 50, 1000)
}

func benchmarkLineEndSplitFunc(b *testing.B, enc encoding.Encoding, encoder func([]byte) []byte, numRecords, recordSize int) {
	// Generate data with LOGEND pattern at the end of each record
	var data []byte
	for i := range numRecords {
		record := make([]byte, recordSize)
		// Fill with 'x'
		for j := 0; j < recordSize-10; j++ {
			record[j] = 'x'
		}
		// End with "_LOGEND_XX" (10 chars)
		suffix := []byte("_LOGEND_")
		copy(record[recordSize-10:], suffix)
		record[recordSize-2] = byte('0' + (i/10)%10)
		record[recordSize-1] = byte('0' + i%10)
		data = append(data, encoder(record)...)
	}

	re := regexp.MustCompile(`(?m)_LOGEND_\d\d`)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		splitFunc := LineEndSplitFunc(re, false, true, enc, nil)
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Split(splitFunc)

		count := 0
		for scanner.Scan() {
			count++
		}
		if scanner.Err() != nil {
			b.Fatal(scanner.Err())
		}
		if count != numRecords {
			b.Fatalf("expected %d records, got %d", numRecords, count)
		}
	}
}

// BenchmarkNewlineSplitFunc_UTF8 benchmarks newline splitting with UTF-8
func BenchmarkNewlineSplitFunc_UTF8(b *testing.B) {
	benchmarkNewlineSplitFunc(b, unicode.UTF8, utf8Encoder, 100, 200)
}

// BenchmarkNewlineSplitFunc_UTF16LE benchmarks newline splitting with UTF-16LE
func BenchmarkNewlineSplitFunc_UTF16LE(b *testing.B) {
	utf16le := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	benchmarkNewlineSplitFunc(b, utf16le, utf16LEEncoder, 100, 200)
}

func benchmarkNewlineSplitFunc(b *testing.B, enc encoding.Encoding, encoder func([]byte) []byte, numRecords, recordSize int) {
	// Generate data with newlines
	var data []byte
	for range numRecords {
		record := make([]byte, recordSize-1)
		for j := range record {
			record[j] = 'x'
		}
		data = append(data, encoder(record)...)
		data = append(data, encoder([]byte("\n"))...)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		splitFunc, _ := NewlineSplitFunc(enc, true)
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Split(splitFunc)

		count := 0
		for scanner.Scan() {
			count++
		}
		if scanner.Err() != nil {
			b.Fatal(scanner.Err())
		}
		if count != numRecords {
			b.Fatalf("expected %d records, got %d", numRecords, count)
		}
	}
}
