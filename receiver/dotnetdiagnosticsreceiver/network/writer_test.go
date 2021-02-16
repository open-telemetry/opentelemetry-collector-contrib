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
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteUTF16String(t *testing.T) {
	buf := &bytes.Buffer{}
	const msg = "hello"
	err := WriteUTF16String(buf, msg)
	require.NoError(t, err)
	p := buf.Bytes()
	buf.Reset()
	var size int32
	err = binary.Read(bytes.NewBuffer(p[:4]), ByteOrder, &size)
	require.NoError(t, err)
	assert.Equal(t, len(msg)+1, int(size))
	assert.Equal(t, 4+2*(len(msg)+1), len(p))
}

func TestWriteUTF16String_Errors(t *testing.T) {
	for i := 0; i < 3; i++ {
		err := WriteUTF16String(&fakeByteBuf{errOn: i}, "hello")
		require.Error(t, err)
	}
}

// fakeByteBuf
type fakeByteBuf struct {
	errOn int
	i     int
}

var _ ByteBuffer = (*fakeByteBuf)(nil)

func (b *fakeByteBuf) Write([]byte) (n int, err error) {
	return 0, b.err()
}

func (b *fakeByteBuf) WriteByte(byte) error {
	return b.err()
}

func (b *fakeByteBuf) err() error {
	defer func() {
		b.i++
	}()
	if b.i == b.errOn {
		return errors.New("")
	}
	return nil
}

func (b *fakeByteBuf) Bytes() []byte {
	return nil
}

func (b *fakeByteBuf) Len() int {
	return 0
}

func (b *fakeByteBuf) Reset() {
}
