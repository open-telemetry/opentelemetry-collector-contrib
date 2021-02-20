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
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderArgs(t *testing.T) {
	a := providerArgs{"foo": "bar", "baz": "glarch"}
	assert.Equal(t, "baz=glarch;foo=bar", a.String())
}

func TestProviderArgs_Escaped(t *testing.T) {
	a := providerArgs{"f;o": "bar", "baz": "gl=rch"}
	assert.Equal(t, `baz="gl=rch";"f;o"=bar`, a.String())
}

func TestProvider_Serialize(t *testing.T) {
	p := createProvider("xx", 42)
	buf := &bytes.Buffer{}
	p.serialize(buf)
	require.Equal(t, 80, len(buf.Bytes()))
}

func TestConfig(t *testing.T) {
	p := provider{
		name:       "System.Runtime",
		eventLevel: verboseEventLevel,
		keywords:   0xffffffff,
		args:       providerArgs{"EventCounterIntervalSec": "1"},
	}
	config := configRequest{
		circularBufferSizeInMB: 10,
		format:                 netTrace,
		requestRundown:         false,
		providers:              []provider{p},
	}
	payload := config.serialize()
	assert.NotNil(t, payload)
	assert.Equal(t, 115, len(payload))
}

func TestRequestHeader_Serialize(t *testing.T) {
	h := requestHeader{
		commandSet: eventPipeCommand,
		commandID:  collectTracing2CommandID,
	}
	p := h.serialize(42)
	assert.Equal(t, []byte(magic), p[:len(magic)])
	assert.Equal(t, requestHeaderSize, len(p))
}

func TestSessionCfg(t *testing.T) {
	req := newConfigRequest(42, "foo")
	payload := req.serialize()
	require.Equal(t, 95, len(payload))
}

func TestRequestWriter_Send(t *testing.T) {
	rw := &fakeRW{}
	w := NewRequestWriter(rw, 0, "")
	err := w.SendRequest()
	require.NoError(t, err)
	require.Equal(t, 107, len(rw.writeBuf))
}

// fakeRW
type fakeRW struct {
	writeBuf []byte
}

var _ io.ReadWriter = (*fakeRW)(nil)

func (rw *fakeRW) Write(p []byte) (n int, err error) {
	rw.writeBuf = append(rw.writeBuf, p...)
	return len(p), nil
}

func (rw *fakeRW) Read(p []byte) (n int, err error) {
	return len(p), nil
}
