// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracesegment // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/tracesegment"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	recvErr "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/errors"
)

// ProtocolSeparator is the character used to split the header and body in an
// X-Ray segment
const ProtocolSeparator = '\n'

// SplitHeaderBody separates header and body from `buf` using a known separator: ProtocolSeparator.
// It returns the body of the segment if:
// 1. header and body can be correctly separated
// 2. header is valid
func SplitHeaderBody(buf []byte) (*Header, []byte, error) {
	if buf == nil {
		return nil, nil, &recvErr.ErrRecoverable{
			Err: errors.New("buffer to split is nil"),
		}
	}

	var headerBytes, bodyBytes []byte
	loc := bytes.IndexByte(buf, byte(ProtocolSeparator))
	if loc == -1 {
		return nil, nil, &recvErr.ErrRecoverable{
			Err: fmt.Errorf("unable to split incoming data as header and segment, incoming bytes: %v", buf),
		}
	}
	headerBytes = buf[0:loc]
	bodyBytes = buf[loc+1:]

	header := Header{}
	err := json.Unmarshal(headerBytes, &header)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid header %w",
			&recvErr.ErrRecoverable{Err: err})
	} else if !header.IsValid() {
		return nil, nil, &recvErr.ErrRecoverable{
			Err: fmt.Errorf("invalid header %+v", header),
		}
	}
	return &header, bodyBytes, nil
}
