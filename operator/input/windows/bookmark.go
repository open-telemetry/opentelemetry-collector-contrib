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

// +build windows

package windows

import (
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
		return fmt.Errorf("failed to convert bookmark xml to utf16: %s", err)
	}

	handle, err := evtCreateBookmark(utf16)
	if err != nil {
		return fmt.Errorf("failed to create bookmark handle from xml: %s", err)
	}

	b.handle = handle
	return nil
}

// Update will update the bookmark using the supplied event.
func (b *Bookmark) Update(event Event) error {
	if b.handle == 0 {
		handle, err := evtCreateBookmark(nil)
		if err != nil {
			return fmt.Errorf("syscall to `EvtCreateBookmark` failed: %s", err)
		}
		b.handle = handle
	}

	if err := evtUpdateBookmark(b.handle, event.handle); err != nil {
		return fmt.Errorf("syscall to `EvtUpdateBookmark` failed: %s", err)
	}

	return nil
}

// Render will render the bookmark as xml.
func (b *Bookmark) Render(buffer Buffer) (string, error) {
	if b.handle == 0 {
		return "", fmt.Errorf("bookmark handle is not open")
	}

	var bufferUsed, propertyCount uint32
	err := evtRender(0, b.handle, EvtRenderBookmark, buffer.SizeBytes(), buffer.FirstByte(), &bufferUsed, &propertyCount)
	if err == ErrorInsufficientBuffer {
		buffer.UpdateSizeBytes(bufferUsed)
		return b.Render(buffer)
	}

	if err != nil {
		return "", fmt.Errorf("syscall to 'EvtRender' failed: %s", err)
	}

	return buffer.ReadString(bufferUsed)
}

// Close will close the bookmark handle.
func (b *Bookmark) Close() error {
	if b.handle == 0 {
		return nil
	}

	if err := evtClose(b.handle); err != nil {
		return fmt.Errorf("failed to close bookmark handle: %s", err)
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
