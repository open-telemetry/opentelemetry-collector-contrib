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
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

func TestParser(t *testing.T) {
	const fastSerialization = "!FastSerialization.1"
	rw := &network.FakeRW{
		WriteErrIdx: -1,
		ReadErrIdx:  -1,
		Responses: map[int][]byte{
			0: []byte(magicTerminated),
			6: []byte("Nettrace"),
			7: {byte(len(fastSerialization))},
			8: []byte(fastSerialization),
		},
	}
	p := NewParser(rw, zap.NewNop())
	err := p.ParseIPC()
	require.NoError(t, err)
	err = p.ParseNettrace()
	require.NoError(t, err)
}
