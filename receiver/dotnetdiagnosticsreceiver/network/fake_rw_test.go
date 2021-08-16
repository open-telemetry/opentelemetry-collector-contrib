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
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeRW_Write(t *testing.T) {
	rw := &FakeRW{
		WriteErrIdx: -1,
	}
	p := []byte{1, 2, 3, 4}
	n, err := rw.Write(p)
	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, p, rw.Writes)
}

func TestFakeRW_WriteErr(t *testing.T) {
	rw := &FakeRW{}
	_, err := rw.Write(nil)
	require.Error(t, err)
}

func TestFakeRW_Read(t *testing.T) {
	resp := []byte{1, 2, 3, 4}
	rw := &FakeRW{
		ReadErrIdx: -1,
		Responses: map[int][]byte{
			0: resp,
		},
	}
	p := make([]byte, 4)
	n, err := rw.Read(p)
	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, resp, p)
}

func TestFakeRW_ReadErr(t *testing.T) {
	rw := &FakeRW{}
	p := make([]byte, 4)
	_, err := rw.Read(p)
	require.Error(t, err)
}

func TestNewDefaultFakeRW(t *testing.T) {
	rw := NewDefaultFakeRW("magic", "nettrace", "fastserialization")
	p := make([]byte, 5)
	_, err := rw.Read(p)
	require.NoError(t, err)
	require.Equal(t, "magic", string(p))
}

func TestBlobReader(t *testing.T) {
	data, err := ReadBlobData(path.Join("..", "testdata"), 2)
	require.NoError(t, err)
	r := NewBlobReader(data)
	p := make([]byte, 6)
	_, err = r.Read(p)
	require.NoError(t, err)
	assert.Equal(t, "DOTNET", string(p))
	_, _ = r.Write([]byte("GOLANG"))
	assert.Equal(t, "GOLANG", string(r.WriteBuf))
	r.ErrOnRead(0)
	_, err = r.Read(p)
	require.Error(t, err)
}

func TestBlobReader_MultiChunkRead(t *testing.T) {
	data, err := ReadBlobData(path.Join("..", "testdata"), 2)
	require.NoError(t, err)
	r := NewBlobReader(data)
	bigSlice := make([]byte, len(data[0])+len(data[1]))
	_, err = r.Read(bigSlice)
	require.NoError(t, err)
	go func() {
		_, err = r.Read(bigSlice)
	}()
	<-r.Gate()
}

func TestReadBlobData_ReadErr(t *testing.T) {
	_, err := ReadBlobData(path.Join("foo", "bar"), 2)
	require.Error(t, err)
}

func TestBlobReader_ReadAllChunks(t *testing.T) {
	data, err := ReadBlobData(path.Join("..", "testdata"), 1)
	require.NoError(t, err)
	r := NewBlobReader(data)
	p := make([]byte, len(data[0]))
	go func() {
		_, err = r.Read(p)
		_, err = r.Read(p)
	}()
	<-r.Gate()
}

func TestBlobReader_StopOnRead(t *testing.T) {
	data, err := ReadBlobData(path.Join("..", "testdata"), 1)
	require.NoError(t, err)
	r := NewBlobReader(data)
	p := make([]byte, len(data[0]))
	r.StopOnRead(0)
	go func() {
		_, _ = r.Read(p)
	}()
	<-r.Gate()
}
