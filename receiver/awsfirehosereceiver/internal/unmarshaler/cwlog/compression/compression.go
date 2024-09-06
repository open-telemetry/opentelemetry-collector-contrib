// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package compression

import (
	"bytes"
	"compress/gzip"
)

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

func Unzip(data []byte) ([]byte, error) {
	b := bytes.NewBuffer(data)

	r, err := gzip.NewReader(b)
	if err != nil {
		return nil, err
	}

	var rv bytes.Buffer
	_, err = rv.ReadFrom(r)
	if err != nil {
		return nil, err
	}

	return rv.Bytes(), nil
}
