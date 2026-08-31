// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package supervisor // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"

import (
	"fmt"
	"net"

	"golang.org/x/sys/windows"
)

// verifyPeerCredentials reads the connecting named pipe peer's kernel-reported
// process ID via GetNamedPipeClientProcessId and enforces that it matches
// wantPID. Windows has no peer UID, so wantUID is ignored; access to the pipe
// is limited by the process-default DACL instead. GetNamedPipeClientProcessId
// only resolves local clients, so a remote (SMB) peer is inherently rejected.
func verifyPeerCredentials(conn net.Conn, _, wantPID int) error {
	pipeConn, ok := conn.(interface{ Fd() uintptr })
	if !ok {
		return fmt.Errorf("connection type %T does not expose a named pipe handle", conn)
	}

	var pid uint32
	if err := windows.GetNamedPipeClientProcessId(windows.Handle(pipeConn.Fd()), &pid); err != nil {
		return fmt.Errorf("read named pipe client pid: %w", err)
	}

	if int(pid) != wantPID {
		return fmt.Errorf("peer pid %d does not match expected collector pid %d", pid, wantPID)
	}
	return nil
}
