// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBookmarkOpenPreexisting(t *testing.T) {
	bookmark := Bookmark{handle: 5}
	err := bookmark.Open("")
	require.ErrorContains(t, err, "bookmark handle is already open")
}

func TestBookmarkOpenInvalidUTF8(t *testing.T) {
	bookmark := NewBookmark()
	invalidUTF8 := "\u0000"
	err := bookmark.Open(invalidUTF8)
	require.ErrorContains(t, err, "failed to convert bookmark xml to utf16")
}

func TestBookmarkOpenSyscallFailure(t *testing.T) {
	bookmark := NewBookmark()
	xml := "<bookmark><\\bookmark>"
	t.Cleanup(mockWithDeferredRestore(&evtCreateBookmark, func(*uint16) (uintptr, error) { return 0, ErrorNotSupported }))
	err := bookmark.Open(xml)
	require.ErrorContains(t, err, "failed to create bookmark handle from xml")
}

func TestBookmarkOpenSuccess(t *testing.T) {
	bookmark := NewBookmark()
	xml := "<bookmark><\\bookmark>"
	t.Cleanup(mockWithDeferredRestore(&evtCreateBookmark, func(*uint16) (uintptr, error) { return 5, nil }))
	err := bookmark.Open(xml)
	require.NoError(t, err)
	require.Equal(t, uintptr(5), bookmark.handle)
}

func TestBookmarkUpdateFailureOnCreateSyscall(t *testing.T) {
	event := NewEvent(1)
	bookmark := NewBookmark()
	t.Cleanup(mockWithDeferredRestore(&evtCreateBookmark, func(*uint16) (uintptr, error) { return 0, ErrorNotSupported }))
	err := bookmark.Update(event)
	require.ErrorContains(t, err, "syscall to `EvtCreateBookmark` failed")
}

func TestBookmarkUpdateFailureOnUpdateSyscall(t *testing.T) {
	event := NewEvent(1)
	bookmark := NewBookmark()
	t.Cleanup(mockWithDeferredRestore(&evtCreateBookmark, func(*uint16) (uintptr, error) { return 1, nil }))
	t.Cleanup(mockWithDeferredRestore(&evtUpdateBookmark, func(_, _ uintptr) error { return ErrorNotSupported }))
	err := bookmark.Update(event)
	require.ErrorContains(t, err, "syscall to `EvtUpdateBookmark` failed")
}

func TestBookmarkUpdateSuccess(t *testing.T) {
	event := NewEvent(1)
	bookmark := NewBookmark()
	t.Cleanup(mockWithDeferredRestore(&evtCreateBookmark, func(*uint16) (uintptr, error) { return 5, nil }))
	t.Cleanup(mockWithDeferredRestore(&evtUpdateBookmark, func(_, _ uintptr) error { return nil }))
	err := bookmark.Update(event)
	require.NoError(t, err)
	require.Equal(t, uintptr(5), bookmark.handle)
}

func TestBookmarkCloseWhenAlreadyClosed(t *testing.T) {
	bookmark := NewBookmark()
	err := bookmark.Close()
	require.NoError(t, err)
}

func TestBookmarkCloseSyscallFailure(t *testing.T) {
	bookmark := Bookmark{handle: 5}
	t.Cleanup(mockWithDeferredRestore(&evtClose, func(uintptr) error { return ErrorNotSupported }))
	err := bookmark.Close()
	require.ErrorContains(t, err, "failed to close bookmark handle")
}

func TestBookmarkCloseSuccess(t *testing.T) {
	bookmark := Bookmark{handle: 5}
	t.Cleanup(mockWithDeferredRestore(&evtClose, func(uintptr) error { return nil }))
	err := bookmark.Close()
	require.NoError(t, err)
	require.Equal(t, uintptr(0), bookmark.handle)
}

func TestBookmarkRenderWhenClosed(t *testing.T) {
	bookmark := NewBookmark()
	buffer := NewBuffer()
	_, err := bookmark.Render(buffer)
	require.ErrorContains(t, err, "bookmark handle is not open")
}

func TestBookmarkRenderInvalidSyscall(t *testing.T) {
	bookmark := Bookmark{handle: 5}
	buffer := NewBuffer()
	t.Cleanup(mockWithDeferredRestore(&evtRender, func(_, _ uintptr, _, _ uint32, _ *byte) (*uint32, error) { return new(uint32), ErrorNotSupported }))
	_, err := bookmark.Render(buffer)
	require.ErrorContains(t, err, "syscall to 'EvtRender' failed")
}
