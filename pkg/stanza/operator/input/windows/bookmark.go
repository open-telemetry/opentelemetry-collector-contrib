// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"errors"
	"fmt"
	"syscall"
)

// Bookmark is a windows event bookmark.
type Bookmark struct {
	handle uintptr
}

// Open will open the bookmark handle using the supplied xml.
func (b *Bookmark) Open(offsetXML string) error {
	if b.handle != 0 {
		return fmt.Errorf("bookmark handle is already open")
	}

	utf16, err := syscall.UTF16PtrFromString(offsetXML)
	if err != nil {
		return fmt.Errorf("failed to convert bookmark xml to utf16: %w", err)
	}

	handle, err := evtCreateBookmark(utf16)
	if err != nil {
		return fmt.Errorf("failed to create bookmark handle from xml: %w", err)
	}

	b.handle = handle
	return nil
}

// Update will update the bookmark using the supplied event.
func (b *Bookmark) Update(event Event) error {
	if b.handle == 0 {
		handle, err := evtCreateBookmark(nil)
		if err != nil {
			return fmt.Errorf("syscall to `EvtCreateBookmark` failed: %w", err)
		}
		b.handle = handle
	}

	if err := evtUpdateBookmark(b.handle, event.handle); err != nil {
		return fmt.Errorf("syscall to `EvtUpdateBookmark` failed: %w", err)
	}

	return nil
}

// Render will render the bookmark as xml.
func (b *Bookmark) Render(buffer Buffer) (string, error) {
	if b.handle == 0 {
		return "", fmt.Errorf("bookmark handle is not open")
	}

	bufferUsed, err := evtRender(0, b.handle, EvtRenderBookmark, buffer.SizeBytes(), buffer.FirstByte())
	if errors.Is(err, ErrorInsufficientBuffer) {
		buffer.UpdateSizeBytes(*bufferUsed)
		return b.Render(buffer)
	}

	if err != nil {
		return "", fmt.Errorf("syscall to 'EvtRender' failed: %w", err)
	}

	return buffer.ReadString(*bufferUsed)
}

// Close will close the bookmark handle.
func (b *Bookmark) Close() error {
	if b.handle == 0 {
		return nil
	}

	if err := evtClose(b.handle); err != nil {
		return fmt.Errorf("failed to close bookmark handle: %w", err)
	}

	b.handle = 0
	return nil
}

// NewBookmark will create a new bookmark with an empty handle.
func NewBookmark() Bookmark {
	return Bookmark{
		handle: 0,
	}
}
