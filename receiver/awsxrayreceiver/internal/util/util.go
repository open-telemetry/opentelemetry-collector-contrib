// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and limitations under the License.

package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	recvErr "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/tracesegment"
)

// ProtocolSeparator is the character used to split the header and body in an
// X-Ray segment
const ProtocolSeparator = '\n'

// SplitHeaderBody separates header and body from `buf` using a known separator: ProtocolSeparator.
// It returns the body of the segment if:
// 1. header and body can be correctly separated
// 2. header is valid
func SplitHeaderBody(buf []byte) (*tracesegment.Header, []byte, error) {
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

	header := tracesegment.Header{}
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
