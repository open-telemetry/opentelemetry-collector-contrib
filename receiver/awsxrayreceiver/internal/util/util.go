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

package util

import (
	"bytes"

	"go.uber.org/zap"
)

// Reference for this port:
// https://github.com/aws/aws-xray-daemon/blob/master/pkg/util/util.go

// ProtocolSeparator is the character used to split the header and body in an
// X-Ray segment
const ProtocolSeparator = "\n"

// SplitHeaderBody separates header and body of buf using provided separator sep, and stores in returnByte.
func SplitHeaderBody(log *zap.Logger, buf, sep *[]byte, returnByte *[][]byte) [][]byte {
	if buf == nil {
		log.Error("buffer to split is nil")
		return nil
	}
	if sep == nil {
		log.Error("separator used to split the buffer is nil")
		return nil
	}
	if returnByte == nil {
		log.Error("buffer used to store splitted result is nil")
		return nil
	}

	separator := *sep
	bufVal := *buf
	lenSeparator := len(separator)
	var header, body []byte
	header = *buf
	for i := 0; i < len(bufVal); i++ {
		if bytes.Equal(bufVal[i:i+lenSeparator], separator) {
			header = bufVal[0:i]
			body = bufVal[i+lenSeparator:]
			break
		}
		if i == len(bufVal)-1 {
			log.Warn("Missing header", zap.ByteString("header", header))
		}
	}
	returnByteVal := *returnByte
	return append(returnByteVal[:0], header, body)
}
