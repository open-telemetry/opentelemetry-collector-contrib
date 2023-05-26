// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package handles

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func NewManager() Manager {
	return &handleCountManager{processInfoGetter: &ntQuerySystemInfoGetter{}}
}

type syscaller interface {
	getSystemProcessInformation() ([]*windows.SYSTEM_PROCESS_INFORMATION, error)
}

type handleCountManager struct {
	handleCounts       map[int64]uint32
	lastRefreshSuccess bool
	processInfoGetter  syscaller
}

func (m *handleCountManager) Refresh() error {
	m.handleCounts = map[int64]uint32{}

	processInfos, err := m.processInfoGetter.getSystemProcessInformation()
	if err != nil {
		m.lastRefreshSuccess = false
		return err
	}

	for _, pInfo := range processInfos {
		m.handleCounts[int64(pInfo.UniqueProcessID)] = pInfo.HandleCount
	}
	m.lastRefreshSuccess = true
	return nil
}

var (
	ErrNoHandleCounts          = errors.New("no handle counts are currently registered")
	ErrNoHandleCountForProcess = errors.New("no handle count for process")
)

func (m *handleCountManager) GetProcessHandleCount(pid int64) (uint32, error) {
	if len(m.handleCounts) == 0 {
		return 0, ErrNoHandleCounts
	}
	handleCount, ok := m.handleCounts[pid]
	if !ok {
		return 0, fmt.Errorf("%w %d", ErrNoHandleCountForProcess, pid)
	}
	return handleCount, nil
}

type ntQuerySystemInfoGetter struct{}

// Uses the `NtQuerySystemInformation` syscall with the SystemProcessInformation info class to get information about every process
// in the system, which includes each process' handle count.
// https://learn.microsoft.com/en-us/windows/win32/api/winternl/nf-winternl-ntquerysysteminformation#systemprocessinformation
func (ntQuerySystemInfoGetter) getSystemProcessInformation() ([]*windows.SYSTEM_PROCESS_INFORMATION, error) {
	// This implementation was borrowed and modified from the only example I found on the internet of anyone traversing the
	// particular output of this syscall.
	// https://cs.opensource.google/go/x/sys/+/master:windows/svc/security.go;l=82;drc=1d35b9e2eb4edf581781c7f3e2a36fac701f0a24

	var currProcess *windows.SYSTEM_PROCESS_INFORMATION

	// This loop performs the syscall. There will be at most 2 iterations of this loop. If the initial buffer size is
	// sufficiently large, the system will respond with the data in the buffer. If it's not large enough, the error
	// lets us know that the buffer was too small and gives us the actual buffer size, at which point we will loop
	// back with a buffer that has exactly as much room as required.
	for infoSize := uint32((unsafe.Sizeof(*currProcess) + unsafe.Sizeof(uintptr(0))) * 1024); ; {
		currProcess = (*windows.SYSTEM_PROCESS_INFORMATION)(unsafe.Pointer(&make([]byte, infoSize)[0]))

		err := windows.NtQuerySystemInformation(
			windows.SystemProcessInformation,
			unsafe.Pointer(currProcess),
			infoSize,
			&infoSize)
		if err == nil {
			break
		}

		// If the error is `STATUS_INFO_LENGTH_MISMATCH`, then we will loop again. We now know the required buffer
		// size, which was returned from the syscall in the infoSize variable.
		if err != windows.STATUS_INFO_LENGTH_MISMATCH {
			return nil, err
		}
	}

	// The data returned from the syscall is not simply an array of SYSTEM_PROCESS_INFORMATION structs (like part of the docs tells you).
	// The structure is actually a SYSTEM_PROCESS_INFORMATION struct followed by a SYSTEM_THREAD_INFORMATION struct for every thread of
	// the process. This means traversing the buffer isn't as simple as grabbing each chunk of sizeof(SYSTEM_PROCESS_INFORMATION) bytes
	// and casting; the next SYSTEM_PROCESS_INFORMATION struct in the buffer is in an indeterminate place.
	//
	// Luckily, each SYSTEM_PROCESS_INFORMATION struct has the field NextEntryOffset. This is actually the number of bytes away that the
	// next SYSTEM_PROCESS_INFORMATION is located, aka after all the threads for the current process struct.
	//
	// To actually get a Go struct out of the bytes from the buffer, we cast an unsafe.Pointer of the current offset to the Go struct type
	// from the windows package (for the first iteration, this was already done at the beginning of the first loop). On the following iteration,
	// we use the NextEntryOffset of the current process struct to calculate where we need to point for the next cast.
	//
	// We continue this process until the NextEntryOffset field is 0, which signifies that this is the last process struct in the buffer.
	processInfos := []*windows.SYSTEM_PROCESS_INFORMATION{}
	for ; ; currProcess = (*windows.SYSTEM_PROCESS_INFORMATION)(unsafe.Pointer(uintptr(unsafe.Pointer(currProcess)) + uintptr(currProcess.NextEntryOffset))) {
		processInfos = append(processInfos, currProcess)
		if currProcess.NextEntryOffset == 0 {
			break
		}
	}

	return processInfos, nil
}
