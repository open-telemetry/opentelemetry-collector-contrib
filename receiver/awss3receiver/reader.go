// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
)

var errUnmarshalSizeLimit = errors.New("decompressed object exceeds s3downloader.max_unmarshal_size")

func readAll(ctx context.Context, reader io.Reader, limit int64) ([]byte, error) {
	if limit == 0 {
		limit = defaultMaxUnmarshalSize
	}
	var buf bytes.Buffer
	err := copyReader(ctx, &buf, io.LimitReader(reader, limit+1))
	if int64(buf.Len()) > limit {
		return nil, errors.Join(err, fmt.Errorf("%w (%d bytes)", errUnmarshalSizeLimit, limit))
	}
	return buf.Bytes(), err
}

func copyReader(ctx context.Context, writer io.Writer, reader io.Reader) error {
	abort := ctx.Done()
	var buf [32 * 1024]byte
	for {
		select {
		case <-abort:
			return ctx.Err()
		default:
		}
		n, readErr := reader.Read(buf[:])
		if n > 0 {
			written, err := writer.Write(buf[:n])
			if err != nil {
				return err
			}
			if written != n {
				return io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			return ctx.Err()
		}
		if readErr != nil {
			return readErr
		}
	}
}
