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

type exception struct {
	// ID – A 64-bit identifier for the exception, unique among segments in the same trace,
	// in 16 hexadecimal digits.
	ID string `json:"id"`

	// Message – The exception message.
	Message string `json:"message,omitempty"`
}

// cause - A cause can be either a 16 character exception ID or an object with the following fields:
type errCause struct {
	// WorkingDirectory – The full path of the working directory when the exception occurred.
	WorkingDirectory string `json:"working_directory"`

	// Exceptions - The array of exception objects.
	Exceptions []exception `json:"exceptions,omitempty"`
}
