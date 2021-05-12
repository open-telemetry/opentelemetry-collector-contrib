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
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

func TestParseSerializationType(t *testing.T) {
	data, err := network.ReadBlobData(path.Join("..", "testdata"), 1)
	require.NoError(t, err)
	rw := network.NewBlobReader(data)
	reader := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err = reader.Seek(61)
	require.NoError(t, err)
	st, err := parseSerializationType(reader)
	require.NoError(t, err)
	assert.EqualValues(t, 4, st.version)
	assert.EqualValues(t, 4, st.minReaderVersion)
	assert.Equal(t, "Trace", st.name)
}

func TestParseSerializationType_Error(t *testing.T) {
	data, err := network.ReadBlobData(path.Join("..", "testdata"), 1)
	require.NoError(t, err)
	for i := 0; i < 6; i++ {
		testParseSerializationTypeErr(t, data, i)
	}
}

func testParseSerializationTypeErr(t *testing.T, data [][]byte, i int) {
	rw := network.NewBlobReader(data)
	reader := network.NewMultiReader(rw, &network.NopBlobWriter{})
	// metadata block is 61 bytes in
	err := reader.Seek(61)
	require.NoError(t, err)
	rw.ErrOnRead(i)
	_, err = parseSerializationType(reader)
	require.Error(t, err)
}
