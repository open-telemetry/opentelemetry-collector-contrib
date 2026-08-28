// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package supervisor // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// verifyPeerCredentials reads the connecting peer's UID via LOCAL_PEERCRED.
// macOS does not expose the peer PID through getsockopt, so wantPID is ignored
// and only the UID is enforced.
func verifyPeerCredentials(conn *net.UnixConn, wantUID, _ int) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("obtain raw unix connection: %w", err)
	}

	var ucred *unix.Xucred
	var sockErr error
	if controlErr := raw.Control(func(fd uintptr) {
		ucred, sockErr = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	}); controlErr != nil {
		return fmt.Errorf("read LOCAL_PEERCRED: %w", controlErr)
	}
	if sockErr != nil {
		return fmt.Errorf("read LOCAL_PEERCRED: %w", sockErr)
	}

	if int(ucred.Uid) != wantUID {
		return fmt.Errorf("peer uid %d does not match expected uid %d", ucred.Uid, wantUID)
	}
	return nil
}
