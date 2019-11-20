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

package translator

import (
	"encoding/hex"

	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
)

// OpenTelemetry Semantic Convention attribute names for error/fault/exception related attributes
const (
	ErrorObjectAttribute  = "error.object"
	ErrorMessageAttribute = "error.message"
	ErrorKindAttribute    = "error.kind"
)

// CauseData provides the shape for unmarshalling data that records exception.
type CauseData struct {
	WorkingDirectory string      `json:"working_directory,omitempty"`
	Paths            []string    `json:"paths,omitempty"`
	Exceptions       []Exception `json:"exceptions,omitempty"`
}

// Exception provides the shape for unmarshalling an exception.
type Exception struct {
	ID      string  `json:"id,omitempty"`
	Type    string  `json:"type,omitempty"`
	Message string  `json:"message,omitempty"`
	Stack   []Stack `json:"stack,omitempty"`
	Remote  bool    `json:"remote,omitempty"`
}

// Stack provides the shape for unmarshalling an stack.
type Stack struct {
	Path  string `json:"path,omitempty"`
	Line  int    `json:"line,omitempty"`
	Label string `json:"label,omitempty"`
}

func makeCause(status *tracepb.Status, attributes map[string]string) (bool, bool, map[string]string, *CauseData) {
	if status.Code == 0 {
		return false, false, attributes, nil
	}
	var (
		filtered    = make(map[string]string)
		cause       *CauseData
		message     = status.GetMessage()
		errorKind   string
		errorObject string
	)

	for key, value := range attributes {
		switch key {
		case ErrorKindAttribute:
			errorKind = value
		case ErrorMessageAttribute:
			if message == "" {
				message = value
			}
		case StatusTextAttribute:
			if message == "" {
				message = value
			}
		case ErrorObjectAttribute:
			errorObject = value
		default:
			filtered[key] = value
		}
	}
	if message == "" {
		message = errorObject
	}

	if message != "" {
		id := NewSegmentID()
		hexID := hex.EncodeToString(id)

		cause = &CauseData{
			Exceptions: []Exception{
				{
					ID:      hexID,
					Type:    errorKind,
					Message: message,
				},
			},
		}
	}

	if isClientError(status.Code) {
		return true, false, filtered, cause
	}
	return false, true, filtered, cause
}

func isClientError(code int32) bool {
	httpStatus := convertToHTTPStatusCode(code)
	return httpStatus >= 400 && httpStatus < 500
}
