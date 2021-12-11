// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver"

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestWatcher_callback(t *testing.T) {
	tempFolder, err := os.MkdirTemp("", "file")
	assert.NoError(t, err)
	wg := sync.WaitGroup{}
	wg.Add(1)
	var paths []string
	w := watcher{
		logger: zap.NewNop(),
		path:   tempFolder,
		callback: func(path string) error {
			found := false
			for _, p := range paths {
				if p == path {
					found = true
					break
				}
			}
			if !found {
				paths = append(paths, path)
				if len(paths) == 3 {
					wg.Done()
				}
			}
			return nil
		},
	}
	err = w.start()
	assert.NoError(t, err)
	ioutil.WriteFile(filepath.Join(tempFolder, "foo"), []byte("foo"), 0600)
	ioutil.WriteFile(filepath.Join(tempFolder, "bar"), []byte("foo"), 0600)
	ioutil.WriteFile(filepath.Join(tempFolder, "foobar"), []byte("foo"), 0600)
	wg.Wait()
	err = w.stop()
	assert.NoError(t, err)
	assert.Contains(t, paths, filepath.Join(tempFolder, "foo"))
	assert.Contains(t, paths, filepath.Join(tempFolder, "bar"))
	assert.Contains(t, paths, filepath.Join(tempFolder, "foobar"))
}
