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
	"io/ioutil"
	"path"
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

// ReadBlobData is intended for reading binary blobs for testing. It reads
// the passed-in number of files using a naming convention and returns them
// as byte arrays for use by a BlobReader.
func ReadBlobData(dir string, numFiles int) ([][]byte, error) {
	var out [][]byte
	for i := 0; i < numFiles; i++ {
		bytes, err := ioutil.ReadFile(blobFile(dir, i))
		if err != nil {
			return nil, err
		}
		out = append(out, bytes)
	}
	return out, nil
}

func blobFile(dir string, i int) string {
	return path.Join(dir, fmt.Sprintf("msg.%d.bin", i))
}

// BlobReader implements io.ReadWriter and can fake a socket from pre-recorded
// binary blobs. It can also return errors for testing purposes.
type BlobReader struct {
	WriteBuf []byte

	currChunk int
	chunks    []*chunk

	errOnRead int
	readCount int

	done chan struct{}
}

var _ io.ReadWriter = (*BlobReader)(nil)

func NewBlobReader(data [][]byte) *BlobReader {
	br := &BlobReader{
		errOnRead: -1,
		done:      make(chan struct{}),
	}
	for _, p := range data {
		br.chunks = append(br.chunks, &chunk{p: p})
	}
	return br
}

func (r *BlobReader) Write(p []byte) (n int, err error) {
	r.WriteBuf = append(r.WriteBuf, p...)
	return len(p), nil
}

// Read reads the appropriate number of bytes into the passed in slice, from the
// member byte arrays, maintaining a count of how many times it was called. If
// the count matches the errOnRead field's value, an error is returned.
func (r *BlobReader) Read(p []byte) (int, error) {
	if r.errOnRead == r.readCount {
		return 0, fmt.Errorf("deliberate err at readCount %d", r.readCount)
	}
	r.readCount++
	tot := r.readChunk(p)
	for tot < len(p) {
		r.currChunk++
		tot += r.readChunk(p[tot:])
	}
	return tot, nil
}

// SetReadError will cause a call to Read to return an error at the specified
// readCount.
func (r *BlobReader) SetReadError(i int) {
	r.readCount = 0
	r.errOnRead = i
}

func (r *BlobReader) readChunk(p []byte) int {
	if r.currChunk == len(r.chunks) {
		r.done <- struct{}{}
		// signal done, now hang
		ch := make(chan struct{})
		<-ch
	}
	return r.chunks[r.currChunk].read(p)
}

func (r *BlobReader) Done() chan struct{} {
	return r.done
}

type chunk struct {
	p []byte
	i int
}

func (c *chunk) read(dst []byte) int {
	n := copy(dst, c.p[c.i:])
	c.i += n
	return n
}
