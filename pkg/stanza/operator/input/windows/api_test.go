// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
