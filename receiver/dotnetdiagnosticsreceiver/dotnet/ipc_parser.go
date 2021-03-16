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

package dotnet

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

type ipcMsg struct {
	header    ipcHeader
	sessionID int64
}

type ipcHeader struct {
	magic      [14]byte
	size       uint16
	commandSet byte
	responseID byte // aka CommandID
	reserved   uint16
}

// Parses and validates an IPC message:
// https://github.com/dotnet/diagnostics/blob/master/documentation/design-docs/ipc-protocol.md
func parseIPC(r network.MultiReader) error {
	ipc, err := doParseIPC(r)
	if err != nil {
		return err
	}
	return validateIPCHeader(ipc.header)
}

func doParseIPC(r network.MultiReader) (ipcMsg, error) {
	ipc := ipcMsg{}
	err := r.Read(&ipc.header.magic)
	if err != nil {
		return ipcMsg{}, err
	}

	err = r.Read(&ipc.header.size)
	if err != nil {
		return ipcMsg{}, err
	}

	err = r.Read(&ipc.header.commandSet)
	if err != nil {
		return ipcMsg{}, err
	}

	err = r.Read(&ipc.header.responseID)
	if err != nil {
		return ipcMsg{}, err
	}

	err = r.Read(&ipc.header.reserved)
	if err != nil {
		return ipcMsg{}, err
	}

	err = r.Read(&ipc.sessionID)
	if err != nil {
		return ipcMsg{}, err
	}

	return ipc, nil
}

const responseError = 0xFF

func validateIPCHeader(hdr ipcHeader) error {
	foundMagic := string(hdr.magic[:13])
	if foundMagic != magic {
		return fmt.Errorf("ipc header: expected magic %q got %q", magic, foundMagic)
	}
	if hdr.responseID == responseError {
		return errors.New("ipc header: got error response")
	}
	return nil
}
