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
	"errors"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestBlobWriter(t *testing.T) {
	maxFiles := 4
	dir := path.Join(os.TempDir(), "blobtest")
	w := NewBlobWriter(dir, maxFiles, zap.NewNop())
	err := w.Init()
	require.NoError(t, err)
	p := make([]byte, 1024)
	size := 10
	for i := 0; i < 7; i++ {
		offset := i * 20
		fill(p, size, offset)
		w.append(p, size)
		fill(p, size, offset+10)
		w.append(p, size)
		w.flush()
	}
	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)
	assert.Equal(t, maxFiles, len(files))
	_ = os.RemoveAll(dir)
}

func fill(p []byte, size, offset int) {
	for i := 0; i < size; i++ {
		p[i] = byte(i + offset)
	}
}

func TestBlobFileWriter_DirExists(t *testing.T) {
	w := newBlobFileWriter("", 0, zap.NewNop())
	w.mkdir = func(s string, mode os.FileMode) error {
		return os.ErrExist
	}
	err := w.Init()
	require.NoError(t, err)
}

func TestBlobFileWriter_ErrorOnRemove(t *testing.T) {
	logger, logs := observer.New(zapcore.WarnLevel)
	w := newBlobFileWriter("", 0, zap.New(logger))
	w.remove = func(string) error {
		return errors.New("")
	}
	w.deleteOldestFile()
	require.Equal(t, 1, logs.Len())
}

func TestBlobFileWriter_ErrorOnWrite(t *testing.T) {
	logger, logs := observer.New(zapcore.WarnLevel)
	w := newBlobFileWriter("", 0, zap.New(logger))
	w.writeFile = func(string, []byte, os.FileMode) error {
		return errors.New("")
	}
	w.writeCurrentFile()
	require.Equal(t, 1, logs.Len())
}

func TestNopBlobWriter(t *testing.T) {
	w := NewBlobWriter("", 0, zap.NewNop())
	err := w.Init()
	require.NoError(t, err)
	w.append(nil, 0)
	w.flush()
}
