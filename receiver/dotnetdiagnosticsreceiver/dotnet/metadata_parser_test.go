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

func TestParseMetadata(t *testing.T) {
	data, err := network.ReadBlobData(path.Join("..", "testdata"), 4)
	require.NoError(t, err)
	rw := network.NewBlobReader(data)
	reader := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err = reader.Seek(159)
	require.NoError(t, err)
	msgs := fieldMetadataMap{}
	err = parseMetadataBlock(reader, msgs)
	require.NoError(t, err)
	assert.Equal(t, 2, len(msgs))

	header := msgs[1].header
	assert.EqualValues(t, 1, header.metadataID)
	assert.Equal(t, "System.Runtime", header.providerName)
	assert.EqualValues(t, 2, header.eventHeaderID)
	assert.Equal(t, "EventCounters", header.eventName)

	assert.EqualValues(t, "Struct", msgs[1].fields[0].fieldType)
	assert.EqualValues(t, "Struct", msgs[1].fields[0].fields[0].fieldType)
	assert.Equal(t, "Payload", msgs[1].fields[0].fields[0].name)
	assert.Equal(t, 12, len(msgs[1].fields[0].fields[0].fields))
	assert.Equal(t, "Name", msgs[1].fields[0].fields[0].fields[0].name)
	assert.EqualValues(t, "String", msgs[1].fields[0].fields[0].fields[0].fieldType)

	assert.Equal(t, 9, len(msgs[2].fields[0].fields[0].fields))
}

func TestParseMetadataErrors(t *testing.T) {
	data, err := network.ReadBlobData(path.Join("..", "testdata"), 4)
	require.NoError(t, err)
	for i := 0; i < 61; i++ {
		testParseMetadataError(t, data, i)
	}
}

func testParseMetadataError(t *testing.T, data [][]byte, i int) {
	rw := network.NewBlobReader(data)
	reader := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err := reader.Seek(159)
	require.NoError(t, err)

	rw.ErrOnRead(i)

	msgs := fieldMetadataMap{}
	err = parseMetadataBlock(reader, msgs)
	require.Error(t, err)
}
