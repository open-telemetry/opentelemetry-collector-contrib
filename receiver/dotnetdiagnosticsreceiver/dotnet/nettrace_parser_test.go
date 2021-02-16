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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

func TestParseNettrace(t *testing.T) {
	rw := &network.FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: []byte(nettraceName),
			1: {byte(len(nettraceSerialization))},
			2: []byte(nettraceSerialization),
		},
	}
	r := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err := parseNettrace(r)
	require.NoError(t, err)
}

func TestParseNettrace_BadHeaderName(t *testing.T) {
	rw := &network.FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: []byte("nettrace"),
		},
	}
	r := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err := parseNettrace(r)
	require.EqualError(t, err, `header name: expected "Nettrace" got "nettrace"`)
}

func TestParseNettrace_BadSerializationName(t *testing.T) {
	serType := "foo"
	rw := &network.FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: []byte(nettraceName),
			1: {byte(len(serType))},
			2: []byte(serType),
		},
	}
	r := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err := parseNettrace(r)
	require.EqualError(t, err, `serialization type: expected "!FastSerialization.1" got "foo"`)
}

func TestParseNettrace_ReadErr(t *testing.T) {
	for i := 0; i < 3; i++ {
		err := testParseNettraceReadErr(i)
		require.Error(t, err)
	}
}

func testParseNettraceReadErr(i int) error {
	rw := &network.FakeRW{
		ReadErrIdx: i,
	}
	r := network.NewMultiReader(rw, &network.NopBlobWriter{})
	return parseNettrace(r)
}
