// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package network

import (
	"fmt"
	"net"
	"os"
	"path"
)

type DialFunc func(network, address string) (net.Conn, error)

type GlobFunc func(pattern string) (matches []string, err error)

// Connect opens an IPC connection to a local dotnet process, given a PID. On
// Mac/Linux, this is a Unix domain socket. Windows (TBD) will use a named pipe.
// DialFunc and GlobFunc are swappable for testing.
func Connect(pid int, dial DialFunc, glob GlobFunc) (net.Conn, error) {
	tmpdir := os.TempDir()
	sf, err := socketFile(pid, tmpdir, glob)
	if err != nil {
		return nil, err
	}
	return dial("unix", sf)
}

// socketFile returns the path to a file on a Linux or macOS host that conforms
// to the naming convention for a Unix domain socket file given a process ID.
// This file is used as a Unix domain socket endpoint to communicate with the
// dotnet process. For more information, please see the README for this
// receiver.
func socketFile(pid int, tempdir string, glob GlobFunc) (string, error) {
	f := fullGlob(pid, tempdir)
	matches, err := glob(f)
	if err != nil {
		return "", err
	}
	l := len(matches)
	switch {
	case l == 0:
		return "", fmt.Errorf("found no matches for pid %d", pid)
	case l > 1:
		return "", fmt.Errorf("found multiple matches for pid %d", pid)
	}
	return matches[0], nil
}

func fullGlob(pid int, tempdir string) string {
	return path.Join(tempdir, globPattern(pid))
}

// globPattern generates a glob pattern that conforms to the .NET diagnostics
// naming convention for a Unix domain socket file for a given process ID. For
// more information, please see the README for this receiver.
func globPattern(pid int) string {
	return fmt.Sprintf("dotnet-diagnostic-%d-*-socket", pid)
}
