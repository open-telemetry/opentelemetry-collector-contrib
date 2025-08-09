// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/parquet/reader"

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
)

// BufferedReader parses parquet inputs from io streams.
type BufferedReader struct {
	cfg          *Config
	fileReader   *file.Reader
	recordReader pqarrow.RecordReader
}

// NewBufferedReader creates a new reader that can decode parquet data from an io.Reader.
// It will return an error if the parquet data stream cannot be read.
// Note: As io.ReadAll is used, the entire data stream would be read into memory, so very large data streams
// may cause memory bottleneck issues.
func NewBufferedReader(r io.Reader, cfg *Config) (*BufferedReader, error) {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 1
	}

	// reads the contents of the reader object into a byte slice
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read data from stream reader: %w", err)
	}

	// defines a memory allocator for allocating memory for Arrow objects
	pool := memory.NewCheckedAllocator(&memory.GoAllocator{})
	// constructs a parquet file reader object from the byte slice data
	pf, err := file.NewParquetReader(bytes.NewReader(data), file.WithReadProps(parquet.NewReaderProperties(pool)))
	if err != nil {
		return nil, fmt.Errorf("failed to create parquet reader: %w", err)
	}

	// constructs a reader for converting to Arrow objects from an existing parquet file reader object
	reader, err := pqarrow.NewFileReader(pf, pqarrow.ArrowReadProperties{
		Parallel:  cfg.ProcessParallel,
		BatchSize: int64(cfg.BatchSize),
	}, pool)
	if err != nil {
		return nil, fmt.Errorf("failed to create pqarrow file reader: %w", err)
	}

	// constructs a record reader that is capable of reding entire sets of arrow records
	rr, err := reader.GetRecordReader(context.Background(), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create parquet record reader: %w", err)
	}

	return &BufferedReader{
		cfg:          cfg,
		recordReader: rr,
		fileReader:   pf,
	}, nil
}

// Next advances the pointer to point to the next record and returns true if the next record exists.
// It will return false if there are no more records to read.
func (sr *BufferedReader) Next() bool {
	return sr.recordReader.Next()
}

// Record reads the current record from the parquet file and returns it as a JSON marshaled byte slice.
// If no more records are available, the []byte slice will be nil and io.EOF will be returned as an error.
// A JSON marshal error will be returned if the record cannot be marshalled.
func (sr *BufferedReader) Record() ([]byte, error) {
	rec := sr.recordReader.Record()
	if rec == nil {
		return nil, fmt.Errorf("reached the end of the record reader: %w", io.EOF)
	}
	defer rec.Release()
	val, err := rec.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON for parquet value: %w", err)
	}

	return val, nil
}

// Close closes the stream reader and releases all resources.
// It will return an error if the fileReader fails to close.
func (sr *BufferedReader) Close() error {
	sr.recordReader.Release()
	return sr.fileReader.Close()
}
