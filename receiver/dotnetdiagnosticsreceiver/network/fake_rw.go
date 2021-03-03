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
	"fmt"
	"io"
)

// FakeRW fakes an io.ReadWriter for testing.
type FakeRW struct {
	WriteErrIdx int
	Writes      []byte
	writeCount  int

	ReadErrIdx int
	Responses  map[int][]byte
	readCount  int
}

var _ io.ReadWriter = (*FakeRW)(nil)

func NewDefaultFakeRW(magic, nettrace, fastSerialization string) *FakeRW {
	return &FakeRW{
		WriteErrIdx: -1,
		ReadErrIdx:  -1,
		Responses: map[int][]byte{
			0: []byte(magic),
			6: []byte(nettrace),
			7: {byte(len(fastSerialization))},
			8: []byte(fastSerialization),
		},
	}
}

func (rw *FakeRW) Write(p []byte) (n int, err error) {
	defer func() { rw.writeCount++ }()
	if rw.writeCount == rw.WriteErrIdx {
		return 0, fmt.Errorf("deliberate error on write %d", rw.writeCount)
	}
	rw.Writes = append(rw.Writes, p...)
	return len(p), nil
}

func (rw *FakeRW) Read(p []byte) (n int, err error) {
	defer func() { rw.readCount++ }()
	if rw.readCount == rw.ReadErrIdx {
		return 0, fmt.Errorf("deliberate error on read %d", rw.readCount)
	}
	resp, ok := rw.Responses[rw.readCount]
	if ok {
		copy(p, resp)
	}
	return len(p), nil
}
