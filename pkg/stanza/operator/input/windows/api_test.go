// Copyright The OpenTelemetry Authors
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

//go:build windows
// +build windows

package windows

import (
	"syscall"
)

// MockProc is a mocked syscall procedure.
type MockProc struct {
	call func(a ...uintptr) (uintptr, uintptr, error)
}

// Call will return the result of the embedded call function.
func (m MockProc) Call(a ...uintptr) (uintptr, uintptr, error) {
	return m.call(a...)
}

// SimpleMockProc returns a mock proc that will always return the supplied arguments when called.
func SimpleMockProc(r1 uintptr, r2 uintptr, err syscall.Errno) SyscallProc {
	return MockProc{
		call: func(a ...uintptr) (uintptr, uintptr, error) {
			return r1, r2, err
		},
	}
}
