// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package compress // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/compress"

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
)

type Compressor func(in []byte) ([]byte, error)

func NewCompressor(format string) (Compressor, error) {
	switch format {
	case "flate":
		return flateCompressor, nil
	case "gzip":
		return gzipCompressor, nil
	case "zlib":
		return zlibCompressor, nil
	case "noop", "none":
		return noopCompressor, nil
	}

	return nil, fmt.Errorf("unknown compression format: %s", format)
}

func flateCompressor(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.BestSpeed)

	_, err := w.Write(in)
	if err != nil {
		return nil, err
	}

	err = w.Flush()
	if err != nil {
		return nil, err
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func gzipCompressor(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)

	_, err := w.Write(in)
	if err != nil {
		return nil, err
	}

	err = w.Flush()
	if err != nil {
		return nil, err
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func zlibCompressor(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, _ := zlib.NewWriterLevel(&buf, zlib.BestSpeed)

	_, err := w.Write(in)
	if err != nil {
		return nil, err
	}

	err = w.Flush()
	if err != nil {
		return nil, err
	}

	err = w.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func noopCompressor(in []byte) ([]byte, error) {
	return in, nil
}
