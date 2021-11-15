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

package filelogexporter

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/golang/groupcache/lru"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

// fileLogExporter is the implementation of file exporter that writes telemetry data to a file
// in Protobuf-JSON format.
type fileLogExporter struct {
	path                 string
	basedirAttributeKey  string
	filenameAttributeKey string
	maxOpenFiles         int
	files                *lru.Cache
	mutex                sync.Mutex
	logger               *zap.Logger
}

func (*fileLogExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *fileLogExporter) ConsumeLogs(_ context.Context, ld pdata.Logs) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		resourceLog := resourceLogs.At(i)
		var resourceBase string
		if baseDir, exists := resourceLog.Resource().Attributes().Get(e.basedirAttributeKey); exists {
			resourceBase = baseDir.AsString()
		}
		ills := resourceLog.InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			libraryLog := ills.At(j)
			lls := libraryLog.Logs()
			for k := 0; k < lls.Len(); k++ {
				log := lls.At(k)
				logBase := resourceBase
				logAttributes := log.Attributes()
				if baseDir, exists := logAttributes.Get(e.basedirAttributeKey); exists {
					logBase = baseDir.AsString()
				}
				fileName, exists := logAttributes.Get(e.filenameAttributeKey)
				if !exists {
					e.logger.Warn("file name attribute '" + e.filenameAttributeKey + "' not found")
					continue
				}
				f, err := e.getOrCreateFile(logBase, fileName.AsString())
				if err != nil {
					return err
				}
				if _, err := io.WriteString(f, log.Body().StringVal()); err != nil {
					return err
				}
				if _, err := io.WriteString(f, "\n"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (e *fileLogExporter) getOrCreateFile(baseDir string, name string) (f io.WriteCloser, err error) {
	var ok bool
	var i interface{}
	path := filepath.Join(e.path, baseDir, filepath.Dir(name))
	fullFileName := filepath.Join(path, filepath.Base(name))
	if i, ok = e.files.Get(fullFileName); !ok {
		err = os.MkdirAll(path, 0755)
		if err != nil {
			return
		}
		e.logger.Info("opening file " + fullFileName)
		i, err = os.OpenFile(fullFileName, os.O_RDWR|os.O_CREATE, 0600)
		if err == nil {
			e.files.Add(fullFileName, i)
		}
	}
	f = i.(io.WriteCloser)
	return
}

func (e *fileLogExporter) Start(context.Context, component.Host) error {
	e.files = &lru.Cache{
		MaxEntries: e.maxOpenFiles,
		OnEvicted: func(key lru.Key, value interface{}) {
			e.logger.Info("closing file " + key.(string))
			err := value.(io.WriteCloser).Close()
			if err != nil {
				e.logger.Error(err.Error())
			}
		},
	}
	var err error
	e.path, err = filepath.Abs(e.path)
	if err != nil {
		return err
	}
	err = os.MkdirAll(e.path, 0755)
	return err
}

// Shutdown stops the exporter and is invoked during shutdown.
func (e *fileLogExporter) Shutdown(context.Context) error {
	e.files.Clear()
	return nil
}
