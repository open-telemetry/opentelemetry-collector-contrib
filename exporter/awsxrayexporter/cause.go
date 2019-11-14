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

package awsxrayexporter

import (
	"encoding/hex"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"os"
	"strings"
)

const (
	ErrorObjectAttribute  = "error.object"
	ErrorMessageAttribute = "error.message"
	ErrorStackAttribute   = "error.stack"
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

func makeCause(status *tracepb.Status, attributes map[string]string) (isError, isFault bool, cause *CauseData) {
	if status.Code == 0 {
		return
	}

	message := status.GetMessage()
	if message == "" {
		message = attributes[ErrorMessageAttribute]
	}
	if message == "" {
		message = attributes[StatusTextAttribute]
	}
	if message == "" {
		message = attributes[ErrorObjectAttribute]
	}

	if message != "" {
		id := make([]byte, 8)
		mutex.Lock()
		r.Read(id) // rand.Read always returns nil
		mutex.Unlock()

		hexID := hex.EncodeToString(id)

		cause = &CauseData{
			Exceptions: []Exception{
				{
					ID:      hexID,
					Type:    attributes[ErrorKindAttribute],
					Message: message,
				},
			},
		}

		stackStr := attributes[ErrorStackAttribute]
		if stackStr != "" {
			cause.Exceptions[0].Stack = parseStackData(stackStr)
		}

		if dir, err := os.Getwd(); err == nil {
			cause.WorkingDirectory = dir
		}
	}

	if isClientError(status.Code) {
		isError = true
		return
	}

	isFault = true
	return
}

func parseStackData(stackStr string) []Stack {
	parts := strings.Split(stackStr, "|")
	stacks := make([]Stack, len(parts))
	for i, part := range parts {
		stacks[i] = Stack{
			Label: part,
		}
	}
	return stacks
}

func isClientError(code int32) bool {
	return false
}
