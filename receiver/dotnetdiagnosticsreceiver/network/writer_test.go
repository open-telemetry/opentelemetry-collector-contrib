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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteUTF16String(t *testing.T) {
	buf := &bytes.Buffer{}
	const msg = "hello"
	WriteUTF16String(buf, msg)
	p := buf.Bytes()
	buf.Reset()
	var size int32
	err := binary.Read(bytes.NewBuffer(p[:4]), ByteOrder, &size)
	require.NoError(t, err)
	assert.Equal(t, len(msg)+1, int(size))
	assert.Equal(t, 4+2*(len(msg)+1), len(p))
}
