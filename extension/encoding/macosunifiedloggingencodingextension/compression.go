// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"fmt"
	"time"

	"github.com/pierrec/lz4/v4"
)

// CompressionStats tracks decompression statistics
type CompressionStats struct {
	TotalChunksProcessed   int
	TotalBytesCompressed   uint64
	TotalBytesDecompressed uint64
	DecompressionTime      time.Duration
	FailedDecompressions   int
	LZ4Chunks              int
	UncompressedChunks     int
}

// GlobalCompressionStats tracks overall compression statistics
var GlobalCompressionStats CompressionStats

// DecompressChunksetData decompresses chunkset data using LZ4 with enhanced error handling
// Based on the rust implementation with additional validation and statistics
func DecompressChunksetData(compressedData []byte, uncompressedSize uint32, algorithm uint32) ([]byte, error) {
	startTime := time.Now()
	defer func() {
		GlobalCompressionStats.DecompressionTime += time.Since(startTime)
		GlobalCompressionStats.TotalChunksProcessed++
		GlobalCompressionStats.TotalBytesCompressed += uint64(len(compressedData))
	}()

	// Validate compression algorithm
	const LZ4_COMPRESSION = 0x100
	if algorithm != LZ4_COMPRESSION {
		GlobalCompressionStats.FailedDecompressions++
		return nil, fmt.Errorf("unsupported compression algorithm: 0x%x (expected LZ4 0x%x)",
			algorithm, LZ4_COMPRESSION)
	}

	// Validate input parameters
	if len(compressedData) == 0 {
		GlobalCompressionStats.FailedDecompressions++
		return nil, fmt.Errorf("empty compressed data")
	}

	if uncompressedSize == 0 || uncompressedSize > 100*1024*1024 { // 100MB limit
		GlobalCompressionStats.FailedDecompressions++
		return nil, fmt.Errorf("invalid uncompressed size: %d bytes", uncompressedSize)
	}

	// Allocate buffer for decompressed data
	decompressedData := make([]byte, uncompressedSize)

	// Decompress using LZ4
	n, err := lz4.UncompressBlock(compressedData, decompressedData)
	if err != nil {
		GlobalCompressionStats.FailedDecompressions++
		return nil, fmt.Errorf("LZ4 decompression failed: %w", err)
	}

	// Validate decompression result
	if n != int(uncompressedSize) {
		GlobalCompressionStats.FailedDecompressions++
		return nil, fmt.Errorf("decompression size mismatch: expected %d, got %d",
			uncompressedSize, n)
	}

	// Update statistics
	GlobalCompressionStats.LZ4Chunks++
	GlobalCompressionStats.TotalBytesDecompressed += uint64(n)

	return decompressedData[:n], nil
}

// ValidateChunksetSignature validates the chunkset compression signature
func ValidateChunksetSignature(signature uint32) (bool, bool, error) {
	const bv41 = 825521762             // "bv41" signature for compressed data
	const bv41Uncompressed = 758412898 // "bv41-" signature for uncompressed data

	switch signature {
	case bv41:
		return true, true, nil // compressed, valid
	case bv41Uncompressed:
		GlobalCompressionStats.UncompressedChunks++
		return false, true, nil // uncompressed, valid
	default:
		return false, false, fmt.Errorf("invalid chunkset signature: 0x%x (expected 0x%x or 0x%x)",
			signature, bv41, bv41Uncompressed)
	}
}

// GetCompressionStats returns the current compression statistics
func GetCompressionStats() CompressionStats {
	return GlobalCompressionStats
}

// ResetCompressionStats resets the compression statistics
func ResetCompressionStats() {
	GlobalCompressionStats = CompressionStats{}
}

// CompressionRatio calculates the compression ratio
func (stats *CompressionStats) CompressionRatio() float64 {
	if stats.TotalBytesCompressed == 0 {
		return 0.0
	}
	return float64(stats.TotalBytesDecompressed) / float64(stats.TotalBytesCompressed)
}

// SuccessRate calculates the decompression success rate
func (stats *CompressionStats) SuccessRate() float64 {
	if stats.TotalChunksProcessed == 0 {
		return 0.0
	}
	successful := stats.TotalChunksProcessed - stats.FailedDecompressions
	return float64(successful) / float64(stats.TotalChunksProcessed) * 100.0
}

// AverageDecompressionSpeed calculates the average decompression speed in MB/s
func (stats *CompressionStats) AverageDecompressionSpeed() float64 {
	if stats.DecompressionTime.Seconds() == 0 {
		return 0.0
	}
	mbProcessed := float64(stats.TotalBytesDecompressed) / (1024 * 1024)
	return mbProcessed / stats.DecompressionTime.Seconds()
}

// SubchunkDecompressionInfo provides detailed information about a specific subchunk decompression
type SubchunkDecompressionInfo struct {
	SubchunkIndex        int
	Start                uint64
	End                  uint64
	UncompressedSize     uint32
	CompressionAlgorithm uint32
	NumberIndexes        uint32
	NumberStringOffsets  uint32
	DecompressionSuccess bool
	DecompressionTime    time.Duration
	ActualSize           int
}

// DecompressWithSubchunkInfo decompresses data and returns detailed subchunk information
func DecompressWithSubchunkInfo(compressedData []byte, subchunk *CatalogSubchunk) ([]byte, *SubchunkDecompressionInfo, error) {
	startTime := time.Now()

	info := &SubchunkDecompressionInfo{
		Start:                subchunk.Start,
		End:                  subchunk.End,
		UncompressedSize:     subchunk.UncompressedSize,
		CompressionAlgorithm: subchunk.CompressionAlgorithm,
		NumberIndexes:        subchunk.NumberIndex,
		NumberStringOffsets:  subchunk.NumberStringOffsets,
	}

	// Attempt decompression
	decompressedData, err := DecompressChunksetData(compressedData, subchunk.UncompressedSize, subchunk.CompressionAlgorithm)

	info.DecompressionTime = time.Since(startTime)
	info.DecompressionSuccess = (err == nil)

	if err != nil {
		return nil, info, err
	}

	info.ActualSize = len(decompressedData)
	return decompressedData, info, nil
}
