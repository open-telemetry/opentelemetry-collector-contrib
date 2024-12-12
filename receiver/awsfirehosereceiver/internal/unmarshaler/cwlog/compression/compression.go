// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package compression // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog/compression"

import (
	"bytes"
	"compress/gzip"
)

// Zip returns a gzip-compressed representation of the input bytes.
func Zip(data []byte) ([]byte, error) {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)

	_, err := w.Write(data)
	if err != nil {
		return nil, err
	}

	//Close handles flushing.
	if err = w.Close(); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// Unzip expects gzip-compressed input bytes and returns their uncompressed form.
func Unzip(data []byte) ([]byte, error) {
	b := bytes.NewBuffer(data)

	r, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var rv bytes.Buffer
	_, err = rv.ReadFrom(r)
	if err != nil {
		return nil, err
	}

	return rv.Bytes(), nil
}
