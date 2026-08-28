// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package supervisor // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"

import (
	"fmt"
	"net"
	"syscall"
)

// verifyPeerCredentials reads the connecting peer's kernel-vouched credentials
// via SO_PEERCRED and enforces that the peer UID matches wantUID and, when
// wantPID is non-zero, that the peer PID matches wantPID.
func verifyPeerCredentials(conn *net.UnixConn, wantUID, wantPID int) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("obtain raw unix connection: %w", err)
	}

	var ucred *syscall.Ucred
	var sockErr error
	if controlErr := raw.Control(func(fd uintptr) {
		ucred, sockErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); controlErr != nil {
		return fmt.Errorf("read SO_PEERCRED: %w", controlErr)
	}
	if sockErr != nil {
		return fmt.Errorf("read SO_PEERCRED: %w", sockErr)
	}

	if int(ucred.Uid) != wantUID {
		return fmt.Errorf("peer uid %d does not match expected uid %d", ucred.Uid, wantUID)
	}
	if wantPID > 0 && int(ucred.Pid) != wantPID {
		return fmt.Errorf("peer pid %d does not match expected collector pid %d", ucred.Pid, wantPID)
	}
	return nil
}
