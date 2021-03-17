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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

func TestIPCParser_NoErrors(t *testing.T) {
	rw := &network.FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: []byte(magicTerminated),
		},
	}
	r := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err := parseIPC(r)
	require.NoError(t, err)
}

func TestIPCParser_BadMagic(t *testing.T) {
	rw := &network.FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: []byte("DOTNET_IPC_V2"),
		},
	}
	err := parseIPC(network.NewMultiReader(rw, &network.NopBlobWriter{}))
	require.EqualError(t, err, `ipc header: expected magic "DOTNET_IPC_V1" got "DOTNET_IPC_V2"`)
}

func TestIPCParser_BadResponseID(t *testing.T) {
	rw := &network.FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: []byte(magic),
			3: {responseError},
		},
	}
	r := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err := parseIPC(r)
	assert.EqualError(t, err, "ipc header: got error response")
}

func TestIPCParser_ReadErrors(t *testing.T) {
	for i := 0; i < 6; i++ {
		testIPCError(t, i)
	}
}

func testIPCError(t *testing.T, idx int) {
	rw := &network.FakeRW{
		ReadErrIdx: idx,
	}
	r := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err := parseIPC(r)
	require.EqualError(t, err, fmt.Sprintf("deliberate error on read %d", idx))
}
