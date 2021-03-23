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
	"context"
	"path"
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
	p := NewParser(
		rw,
		func(metrics []Metric) {},
		&network.NopBlobWriter{},
		zap.NewNop(),
	)
	err := p.ParseIPC()
	require.NoError(t, err)
	err = p.ParseNettrace()
	require.NoError(t, err)
}

func TestParser_TestData(t *testing.T) {
	data, err := network.ReadBlobData(path.Join("..", "testdata"), 16)
	require.NoError(t, err)
	rw := network.NewBlobReader(data)
	parser := NewParser(rw, func([]Metric) {}, &network.NopBlobWriter{}, zap.NewNop())
	err = parser.ParseIPC()
	require.NoError(t, err)
	err = parser.ParseNettrace()
	require.NoError(t, err)
	msgs := fieldMetadataMap{}
	for i := 0; i < 16; i++ {
		err = parser.parseBlock(msgs)
		require.NoError(t, err)
	}
}

func TestParser_TestData_Errors(t *testing.T) {
	data, err := network.ReadBlobData(path.Join("..", "testdata"), 16)
	require.NoError(t, err)
	// beginPrivateObject
	testParseBlockError(t, data, 60, 0)
	// parseSerializationType
	testParseBlockError(t, data, 60, 1)
	// parseTraceMessage
	testParseBlockError(t, data, 60, 8)
	// parseMetadataBlock
	testParseBlockError(t, data, 130, 8)
	// parseStackBlock
	testParseBlockError(t, data, 946, 8)
	// parseEventBlock
	testParseBlockError(t, data, 1105, 8)
	// parseSPBlock
	testParseBlockError(t, data, 36847, 8)
}

func testParseBlockError(t *testing.T, data [][]byte, offset, errIdx int) {
	err := testParseBlock(t, data, offset, errIdx)
	require.Error(t, err)
}

func testParseBlock(t *testing.T, data [][]byte, offset, errIdx int) error {
	rw := network.NewBlobReader(data)
	reader := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err := reader.Seek(offset)
	require.NoError(t, err)
	rw.ErrOnRead(errIdx)
	msgs := fieldMetadataMap{}
	parser := &Parser{r: reader, logger: zap.NewNop()}
	for i := 0; i < 16; i++ {
		err := parser.parseBlock(msgs)
		if err != nil {
			return err
		}
	}
	return nil
}

func TestParser_ParseAll_Error(t *testing.T) {
	data, err := network.ReadBlobData(path.Join("..", "testdata"), 16)
	require.NoError(t, err)
	rw := network.NewBlobReader(data)
	parser := NewParser(rw, func([]Metric) {}, &network.NopBlobWriter{}, zap.NewNop())
	err = parser.ParseIPC()
	require.NoError(t, err)
	err = parser.ParseNettrace()
	require.NoError(t, err)
	rw.StopOnRead(0)
	errCh := make(chan error)
	go func() {
		err = parser.ParseAll(context.Background())
		errCh <- err
	}()
	<-rw.Gate()
	rw.ErrOnRead(0)
	rw.Gate() <- struct{}{}
	require.Error(t, <-errCh)
}

func TestParser_ParseAll_Cancel(t *testing.T) {
	data, err := network.ReadBlobData(path.Join("..", "testdata"), 16)
	require.NoError(t, err)
	rw := network.NewBlobReader(data)
	parser := NewParser(rw, func([]Metric) {}, &network.NopBlobWriter{}, zap.NewNop())
	err = parser.ParseIPC()
	require.NoError(t, err)
	err = parser.ParseNettrace()
	require.NoError(t, err)
	rw.StopOnRead(0)
	errCh := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err = parser.ParseAll(ctx)
		errCh <- err
	}()
	<-rw.Gate()
	cancel()
	rw.Gate() <- struct{}{}
	require.NoError(t, <-errCh)
}
