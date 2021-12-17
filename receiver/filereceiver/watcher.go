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
	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

type watcher struct {
	logger   *zap.Logger
	path     string
	watcher  *fsnotify.Watcher
	callback func(path string) error
}

func (w *watcher) start() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.watcher = watcher

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				path := event.Name
				if event.Op == fsnotify.Write || event.Op == fsnotify.Create {
					err := w.callback(path)
					w.logger.Error("error reading", zap.String("path", w.path), zap.Error(err))
				}

			case err := <-watcher.Errors:
				w.logger.Error("error watching ", zap.String("path", w.path), zap.Error(err))
			}
		}
	}()

	if err := watcher.Add(w.path); err != nil {
		return err
	}

	return nil
}

func (w *watcher) stop() error {
	return w.watcher.Close()
}
