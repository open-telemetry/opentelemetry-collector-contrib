// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	api = windows.NewLazySystemDLL("wevtapi.dll")

	subscribeProc             SyscallProc = api.NewProc("EvtSubscribe")
	nextProc                  SyscallProc = api.NewProc("EvtNext")
	renderProc                SyscallProc = api.NewProc("EvtRender")
	closeProc                 SyscallProc = api.NewProc("EvtClose")
	createBookmarkProc        SyscallProc = api.NewProc("EvtCreateBookmark")
	updateBookmarkProc        SyscallProc = api.NewProc("EvtUpdateBookmark")
	openPublisherMetadataProc SyscallProc = api.NewProc("EvtOpenPublisherMetadata")
	formatMessageProc         SyscallProc = api.NewProc("EvtFormatMessage")
)

// SyscallProc is a syscall procedure.
type SyscallProc interface {
	Call(...uintptr) (uintptr, uintptr, error)
}

const (
	// EvtSubscribeToFutureEvents is a flag that will subscribe to only future events.
	EvtSubscribeToFutureEvents uint32 = 1
	// EvtSubscribeStartAtOldestRecord is a flag that will subscribe to all existing and future events.
	EvtSubscribeStartAtOldestRecord uint32 = 2
	// EvtSubscribeStartAfterBookmark is a flag that will subscribe to all events that begin after a bookmark.
	EvtSubscribeStartAfterBookmark uint32 = 3
)

const (
	// ErrorSuccess is an error code that indicates the operation completed successfully.
	ErrorSuccess syscall.Errno = 0
	// ErrorNotSupported is an error code that indicates the operation is not supported.
	ErrorNotSupported syscall.Errno = 50
	// ErrorInsufficientBuffer is an error code that indicates the data area passed to a system call is too small
	ErrorInsufficientBuffer syscall.Errno = 122
	// ErrorNoMoreItems is an error code that indicates no more items are available.
	ErrorNoMoreItems syscall.Errno = 259
	// ErrorInvalidOperation is an error code that indicates the operation identifier is not valid
	ErrorInvalidOperation syscall.Errno = 4317
)

const (
	// EvtFormatMessageXML is flag that formats a message as an XML string that contains all event details and message strings.
	EvtFormatMessageXML uint32 = 9
)

const (
	// EvtRenderEventXML is a flag to render an event as an XML string
	EvtRenderEventXML uint32 = 1
	// EvtRenderBookmark is a flag to render a bookmark as an XML string
	EvtRenderBookmark uint32 = 2
)

// evtSubscribe is the direct syscall implementation of EvtSubscribe (https://docs.microsoft.com/en-us/windows/win32/api/winevt/nf-winevt-evtsubscribe)
func evtSubscribe(session uintptr, signalEvent windows.Handle, channelPath *uint16, query *uint16, bookmark uintptr, context uintptr, callback uintptr, flags uint32) (uintptr, error) {
	handle, _, err := subscribeProc.Call(session, uintptr(signalEvent), uintptr(unsafe.Pointer(channelPath)), uintptr(unsafe.Pointer(query)), bookmark, context, callback, uintptr(flags))
	if !errors.Is(err, ErrorSuccess) {
		return 0, err
	}

	return handle, nil
}

// evtNext is the direct syscall implementation of EvtNext (https://docs.microsoft.com/en-us/windows/win32/api/winevt/nf-winevt-evtnext)
func evtNext(resultSet uintptr, eventsSize uint32, events *uintptr, timeout uint32, flags uint32, returned *uint32) error {
	_, _, err := nextProc.Call(resultSet, uintptr(eventsSize), uintptr(unsafe.Pointer(events)), uintptr(timeout), uintptr(flags), uintptr(unsafe.Pointer(returned)))
	if !errors.Is(err, ErrorSuccess) {
		return err
	}

	return nil
}

// evtRender is the direct syscall implementation of EvtRender (https://docs.microsoft.com/en-us/windows/win32/api/winevt/nf-winevt-evtrender)
func evtRender(context uintptr, fragment uintptr, flags uint32, bufferSize uint32, buffer *byte) (*uint32, error) {
	bufferUsed := new(uint32)
	propertyCount := new(uint32)
	_, _, err := renderProc.Call(context, fragment, uintptr(flags), uintptr(bufferSize), uintptr(unsafe.Pointer(buffer)), uintptr(unsafe.Pointer(bufferUsed)), uintptr(unsafe.Pointer(propertyCount)))
	if !errors.Is(err, ErrorSuccess) {
		return bufferUsed, err
	}

	return bufferUsed, nil
}

// evtClose is the direct syscall implementation of EvtClose (https://docs.microsoft.com/en-us/windows/win32/api/winevt/nf-winevt-evtclose)
func evtClose(handle uintptr) error {
	_, _, err := closeProc.Call(handle)
	if !errors.Is(err, ErrorSuccess) {
		return err
	}

	return nil
}

// evtCreateBookmark is the direct syscall implementation of EvtCreateBookmark (https://docs.microsoft.com/en-us/windows/win32/api/winevt/nf-winevt-evtcreatebookmark)
func evtCreateBookmark(bookmarkXML *uint16) (uintptr, error) {
	handle, _, err := createBookmarkProc.Call(uintptr(unsafe.Pointer(bookmarkXML)))
	if !errors.Is(err, ErrorSuccess) {
		return 0, err
	}

	return handle, nil
}

// evtUpdateBookmark is the direct syscall implementation of EvtUpdateBookmark (https://docs.microsoft.com/en-us/windows/win32/api/winevt/nf-winevt-evtcreatebookmark)
func evtUpdateBookmark(bookmark uintptr, event uintptr) error {
	_, _, err := updateBookmarkProc.Call(bookmark, event)
	if !errors.Is(err, ErrorSuccess) {
		return err
	}

	return nil
}

// evtOpenPublisherMetadata is the direct syscall implementation of EvtOpenPublisherMetadata (https://docs.microsoft.com/en-us/windows/win32/api/winevt/nf-winevt-evtopenpublishermetadata)
func evtOpenPublisherMetadata(session uintptr, publisherIdentity *uint16, logFilePath *uint16, locale uint32, flags uint32) (uintptr, error) {
	handle, _, err := openPublisherMetadataProc.Call(session, uintptr(unsafe.Pointer(publisherIdentity)), uintptr(unsafe.Pointer(logFilePath)), uintptr(locale), uintptr(flags))
	if !errors.Is(err, ErrorSuccess) {
		return 0, err
	}

	return handle, nil
}

// evtFormatMessage is the direct syscall implementation of EvtFormatMessage (https://docs.microsoft.com/en-us/windows/win32/api/winevt/nf-winevt-evtformatmessage)
func evtFormatMessage(publisherMetadata uintptr, event uintptr, messageID uint32, valueCount uint32, values uintptr, flags uint32, bufferSize uint32, buffer *byte) (*uint32, error) {
	bufferUsed := new(uint32)
	_, _, err := formatMessageProc.Call(publisherMetadata, event, uintptr(messageID), uintptr(valueCount), values, uintptr(flags), uintptr(bufferSize), uintptr(unsafe.Pointer(buffer)), uintptr(unsafe.Pointer(bufferUsed)))
	if !errors.Is(err, ErrorSuccess) {
		return bufferUsed, err
	}

	return bufferUsed, nil
}
