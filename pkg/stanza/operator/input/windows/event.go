// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"errors"
	"fmt"
	"unicode/utf16"
	"unsafe"
)

// systemPropertiesRenderContext stores a custom rendering context to get only the event properties.
var (
	systemPropertiesRenderContext    = uintptr(0)
	systemPropertiesRenderContextErr error
)

func init() {
	// This is not expected to fail, however, collecting the error if a new failure mode appears.
	systemPropertiesRenderContext, systemPropertiesRenderContextErr = evtCreateRenderContext(0, nil, EvtRenderContextSystem)
}

// Event is an event stored in windows event log.
type Event struct {
	handle uintptr
}

// GetPublisherName will get the publisher name of the event.
func (e *Event) GetPublisherName(buffer *Buffer) (string, error) {
	if e.handle == 0 {
		return "", errors.New("event handle does not exist")
	}

	if systemPropertiesRenderContextErr != nil {
		return "", fmt.Errorf("failed to create render context: %w", systemPropertiesRenderContextErr)
	}

	bufferUsed, err := evtRender(systemPropertiesRenderContext, e.handle, EvtRenderEventValues, buffer.SizeBytes(), buffer.FirstByte())
	if errors.Is(err, ErrorInsufficientBuffer) {
		buffer.UpdateSizeBytes(*bufferUsed)
		return e.GetPublisherName(buffer)
	}

	if err != nil {
		return "", fmt.Errorf("failed to get provider name: %w", err)
	}

	utf16Ptr := (**uint16)(unsafe.Pointer(buffer.FirstByte()))
	providerName := utf16PtrToString(*utf16Ptr)

	return providerName, nil
}

// utf16PtrToString converts Windows API LPWSTR (pointer to string) to go string
func utf16PtrToString(s *uint16) string {
	if s == nil {
		return ""
	}

	utf16Len := 0
	curPtr := unsafe.Pointer(s)
	for *(*uint16)(curPtr) != 0 {
		curPtr = unsafe.Pointer(uintptr(curPtr) + unsafe.Sizeof(*s))
		utf16Len++
	}

	slice := unsafe.Slice(s, utf16Len)
	return string(utf16.Decode(slice))
}

// NewEvent will create a new event from an event handle.
func NewEvent(handle uintptr) Event {
	return Event{
		handle: handle,
	}
}

// RenderSimple will render the event as EventXML without formatted info.
func (e *Event) RenderSimple(buffer *Buffer) (*EventXML, error) {
	if e.handle == 0 {
		return nil, errors.New("event handle does not exist")
	}

	bufferUsed, err := evtRender(0, e.handle, EvtRenderEventXML, buffer.SizeBytes(), buffer.FirstByte())
	if err != nil {
		if errors.Is(err, ErrorInsufficientBuffer) {
			buffer.UpdateSizeBytes(*bufferUsed)
			return e.RenderSimple(buffer)
		}
		return nil, fmt.Errorf("syscall to 'EvtRender' failed: %w", err)
	}

	bytes, err := buffer.ReadBytes(*bufferUsed)
	if err != nil {
		return nil, fmt.Errorf("failed to read bytes from buffer: %w", err)
	}

	return unmarshalEventXML(bytes)
}

// RenderDeep will render the event as EventXML with all available formatted info.
func (e *Event) RenderDeep(buffer *Buffer, publisher Publisher) (*EventXML, error) {
	if e.handle == 0 {
		return nil, errors.New("event handle does not exist")
	}

	bufferUsed, err := evtFormatMessage(publisher.handle, e.handle, 0, 0, 0, EvtFormatMessageXML, buffer.SizeWide(), buffer.FirstByte())
	if err != nil {
		if errors.Is(err, ErrorInsufficientBuffer) {
			buffer.UpdateSizeWide(*bufferUsed)
			return e.RenderDeep(buffer, publisher)
		}
		return nil, fmt.Errorf("syscall to 'EvtFormatMessage' failed: %w", err)
	}

	bytes, err := buffer.ReadWideChars(*bufferUsed)
	if err != nil {
		return nil, fmt.Errorf("failed to read bytes from buffer: %w", err)
	}

	return unmarshalEventXML(bytes)
}

// Close will close the event handle.
func (e *Event) Close() error {
	if e.handle == 0 {
		return nil
	}

	if err := evtClose(e.handle); err != nil {
		return fmt.Errorf("failed to close event handle: %w", err)
	}

	e.handle = 0
	return nil
}
