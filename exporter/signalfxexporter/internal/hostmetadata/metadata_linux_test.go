// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetadata

import "golang.org/x/sys/unix"

func mockSyscallUname() {
	syscallUname = func(in *unix.Utsname) error {
		in.Machine = [65]byte{}
		return nil
	}
}
