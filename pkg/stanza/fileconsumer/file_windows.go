// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
	"os"
	"syscall"
)

// Noop on windows because we close files immediately after reading.
func (*Manager) readLostFiles(context.Context) {
}

// openFile opens a file on Windows with FILE_SHARE_DELETE flag to allow
// other processes to delete or rename the file while it's open.
func openFile(path string) (*os.File, error) {
	if path == "" {
		return nil, syscall.ERROR_FILE_NOT_FOUND
	}

	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	access := uint32(syscall.GENERIC_READ)
	sharemode := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE)
	createmode := uint32(syscall.OPEN_EXISTING)
	flags := uint32(syscall.FILE_ATTRIBUTE_NORMAL)

	handle, err := syscall.CreateFile(pathp, access, sharemode, nil, createmode, flags, 0)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(handle), path), nil
}
