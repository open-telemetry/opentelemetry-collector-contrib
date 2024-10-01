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
	createBookmarkProc = SimpleMockProc(0, 0, ErrorNotSupported)
	err := bookmark.Open(xml)
	require.ErrorContains(t, err, "failed to create bookmark handle from xml")
}

func TestBookmarkOpenSuccess(t *testing.T) {
	bookmark := NewBookmark()
	xml := "<bookmark><\\bookmark>"
	createBookmarkProc = SimpleMockProc(5, 0, ErrorSuccess)
	err := bookmark.Open(xml)
	require.NoError(t, err)
	require.Equal(t, uintptr(5), bookmark.handle)
}

func TestBookmarkUpdateFailureOnCreateSyscall(t *testing.T) {
	event := NewEvent(1)
	bookmark := NewBookmark()
	createBookmarkProc = SimpleMockProc(0, 0, ErrorNotSupported)
	err := bookmark.Update(event)
	require.ErrorContains(t, err, "syscall to `EvtCreateBookmark` failed")
}

func TestBookmarkUpdateFailureOnUpdateSyscall(t *testing.T) {
	event := NewEvent(1)
	bookmark := NewBookmark()
	createBookmarkProc = SimpleMockProc(1, 0, ErrorSuccess)
	updateBookmarkProc = SimpleMockProc(0, 0, ErrorNotSupported)
	err := bookmark.Update(event)
	require.ErrorContains(t, err, "syscall to `EvtUpdateBookmark` failed")
}

func TestBookmarkUpdateSuccess(t *testing.T) {
	event := NewEvent(1)
	bookmark := NewBookmark()
	createBookmarkProc = SimpleMockProc(5, 0, ErrorSuccess)
	updateBookmarkProc = SimpleMockProc(1, 0, ErrorSuccess)
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
	closeProc = SimpleMockProc(0, 0, ErrorNotSupported)
	err := bookmark.Close()
	require.ErrorContains(t, err, "failed to close bookmark handle")
}

func TestBookmarkCloseSuccess(t *testing.T) {
	bookmark := Bookmark{handle: 5}
	closeProc = SimpleMockProc(1, 0, ErrorSuccess)
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
	renderProc = SimpleMockProc(0, 0, ErrorNotSupported)
	_, err := bookmark.Render(buffer)
	require.ErrorContains(t, err, "syscall to 'EvtRender' failed")
}
