// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows

import (
	"encoding/hex"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// canaryValue is 13 bytes, so every clone lands in a 16-byte tiny-allocator block: the same size class
// as a short UTF-16 string handed to the Windows Event Log API.
const canaryValue = "canary-string"

// addrToPointer converts an integer address back to a pointer, standing in for the kernel side of a
// Win32 call that reads through an argument it received as a plain integer.
func addrToPointer(addr uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&addr)) }

// staleReadProc stands in for EvtCreateBookmark as the kernel sees it: it receives the address of the
// bookmark XML as a plain integer, the Go heap keeps moving while the call is in flight, and only then
// does it read the string. If the caller's UTF-16 buffer was collected in the meantime, the read sees
// whatever reused the memory.
type staleReadProc struct {
	want   []uint16
	keep   []string
	churns int
	calls  int
	stale  int
	sample string
}

func (p *staleReadProc) Call(a ...uintptr) (uintptr, uintptr, error) {
	xml := a[0]
	runtime.GC()
	runtime.GC()
	for i := 0; i < p.churns; i++ {
		p.keep = append(p.keep, strings.Clone(canaryValue))
	}
	got := unsafe.Slice((*uint16)(addrToPointer(xml)), len(p.want))
	p.calls++
	if !slices.Equal(got, p.want) {
		p.stale++
		if p.sample == "" {
			p.sample = hex.EncodeToString(unsafe.Slice((*byte)(addrToPointer(xml)), 2*len(p.want)))
		}
	}
	return 1, 0, nil
}

// TestBookmarkOpenKeepsXMLAliveDuringCall checks that the UTF-16 bookmark XML passed to EvtCreateBookmark
// stays valid until the call returns. The pointer crosses the SyscallProc interface as a uintptr, so the
// compiler does not extend its lifetime on its own.
func TestBookmarkOpenKeepsXMLAliveDuringCall(t *testing.T) {
	orig := createBookmarkProc
	t.Cleanup(func() { createBookmarkProc = orig })

	const xml = "<xml/>" // 7 UTF-16 code units including the terminator: a tiny-allocator block
	want, err := windows.UTF16FromString(xml)
	require.NoError(t, err)
	proc := &staleReadProc{want: want, churns: 20000}
	createBookmarkProc = proc

	for i := 0; i < 200; i++ {
		var b Bookmark
		require.NoError(t, b.Open(xml))
	}

	t.Logf("EvtCreateBookmark calls=%d stale reads=%d sample=%s", proc.calls, proc.stale, proc.sample)
	require.Zero(t, proc.stale, "bookmark XML was freed before EvtCreateBookmark finished reading it (%d of %d calls, e.g. %s)", proc.stale, proc.calls, proc.sample)
}
