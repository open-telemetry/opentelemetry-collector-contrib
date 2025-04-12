// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver/internal"

import (
	"bytes"
	"fmt"
	"io"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
)

const messageSizeLargerErrFmt = "received message larger than max (%d vs %d)"

// parseProtoReader parses a compressed proto from an io.Reader.
func parseProtoReader(reader io.Reader, expectedSize, maxSize int, req proto.Message) error {
	body, err := decompressRequest(reader, expectedSize, maxSize)
	if err != nil {
		return err
	}

	// We re-implement proto.Unmarshal here as it calls XXX_Unmarshal first,
	// which we can't override without upsetting golint.
	req.Reset()
	if u, ok := req.(proto.Unmarshaler); ok {
		err = u.Unmarshal(body)
	} else {
		err = proto.NewBuffer(body).Unmarshal(req)
	}
	if err != nil {
		return err
	}

	return nil
}

func decompressRequest(reader io.Reader, expectedSize, maxSize int) (body []byte, err error) {
	defer func() {
		if err != nil && len(body) > maxSize {
			err = fmt.Errorf(messageSizeLargerErrFmt, len(body), maxSize)
		}
	}()
	if expectedSize > maxSize {
		return nil, fmt.Errorf(messageSizeLargerErrFmt, expectedSize, maxSize)
	}
	buffer, ok := tryBufferFromReader(reader)
	if ok {
		body, err = decompressFromBuffer(buffer, maxSize)
		return
	}
	body, err = decompressFromReader(reader, expectedSize, maxSize)
	return
}

func decompressFromReader(reader io.Reader, expectedSize, maxSize int) ([]byte, error) {
	var (
		buf  bytes.Buffer
		body []byte
		err  error
	)
	if expectedSize > 0 {
		buf.Grow(expectedSize + bytes.MinRead) // extra space guarantees no reallocation
	}
	// Read from LimitReader with limit max+1. So if the underlying
	// reader is over limit, the result will be bigger than max.
	reader = io.LimitReader(reader, int64(maxSize)+1)
	_, err = buf.ReadFrom(reader)
	if err != nil {
		return nil, err
	}
	body, err = decompressFromBuffer(&buf, maxSize)

	return body, err
}

func decompressFromBuffer(buffer *bytes.Buffer, maxSize int) ([]byte, error) {
	if len(buffer.Bytes()) > maxSize {
		return nil, fmt.Errorf(messageSizeLargerErrFmt, len(buffer.Bytes()), maxSize)
	}
	size, err := snappy.DecodedLen(buffer.Bytes())
	if err != nil {
		return nil, err
	}
	if size > maxSize {
		return nil, fmt.Errorf(messageSizeLargerErrFmt, size, maxSize)
	}
	body, err := snappy.Decode(nil, buffer.Bytes())
	if err != nil {
		return nil, err
	}
	return body, nil
}

// tryBufferFromReader attempts to cast the reader to a `*bytes.Buffer` this is possible when using httpgrpc.
// If it fails it will return nil and false.
func tryBufferFromReader(reader io.Reader) (*bytes.Buffer, bool) {
	if bufReader, ok := reader.(interface {
		BytesBuffer() *bytes.Buffer
	}); ok && bufReader != nil {
		return bufReader.BytesBuffer(), true
	}
	return nil, false
}
