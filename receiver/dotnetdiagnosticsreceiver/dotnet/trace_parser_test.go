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

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

func TestTraceParser(t *testing.T) {
	data, err := network.ReadBlobData(path.Join("..", "testdata"), 2)
	require.NoError(t, err)
	rw := network.NewBlobReader(data)
	reader := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err = reader.Seek(81)
	require.NoError(t, err)
	err = parseTraceMessage(reader)
	require.NoError(t, err)
	require.Equal(t, 129, reader.Pos())
}

func TestTraceParser_Errors(t *testing.T) {
	data, err := network.ReadBlobData(path.Join("..", "testdata"), 2)
	require.NoError(t, err)
	for i := 0; i < 7; i++ {
		testTraceParserReadError(t, data, i)
	}
}

func testTraceParserReadError(t *testing.T, data [][]byte, i int) {
	rw := network.NewBlobReader(data)
	reader := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err := reader.Seek(81)
	require.NoError(t, err)
	rw.ErrOnRead(i)
	err = parseTraceMessage(reader)
	require.Error(t, err)
}
