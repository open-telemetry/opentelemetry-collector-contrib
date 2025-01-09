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

	if err = w.Flush(); err != nil {
		return nil, err
	}

	if err = w.Close(); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}
