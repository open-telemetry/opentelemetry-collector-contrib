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

package otlpjsonfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"

import (
	"bufio"
	"context"
	"os"
	"regexp"

	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var (
	tracesUnmarshaler  = ptrace.NewJSONUnmarshaler()
	logsUnmarshaler    = plog.NewJSONUnmarshaler()
	metricsUnmarshaler = pmetric.NewJSONUnmarshaler()
)

type watcher struct {
	logger          *zap.Logger
	path            string
	fswatcher       *fsnotify.Watcher
	logsConsumer    consumer.Logs
	tracesConsumer  consumer.Traces
	metricsConsumer consumer.Metrics
	done            chan bool
}

var (
	logsFingerPrint    = regexp.MustCompile(`{\s*"resourceLogs"`)
	metricsFingerPrint = regexp.MustCompile(`{\s*"resourceMetrics"`)
	tracesFingerPrint  = regexp.MustCompile(`{\s*"resourceSpans"`)
)

func (w *watcher) start(ctx context.Context) error {
	fswatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.fswatcher = fswatcher
	w.done = make(chan bool, 1)

	go func() {
		for {
			select {
			case event := <-fswatcher.Events:
				path := event.Name
				if event.Op == fsnotify.Write || event.Op == fsnotify.Create {
					err := w.processFile(path)
					w.logger.Error("error reading", zap.String("path", w.path), zap.Error(err))
				}
			case <-w.done:
				return
			case <-ctx.Done():
				return
			case err := <-fswatcher.Errors:
				w.logger.Error("error watching ", zap.String("path", w.path), zap.Error(err))
			}
		}
	}()

	if err := fswatcher.Add(w.path); err != nil {
		return err
	}

	return nil
}

func (w *watcher) processFile(path string) error {
	fileHandle, _ := os.Open(path)
	defer fileHandle.Close()
	fileScanner := bufio.NewScanner(fileHandle)

	var allErrors []error
	for fileScanner.Scan() {
		if err := fileScanner.Err(); err != nil {
			allErrors = append(allErrors, err)
			break
		}
		line := fileScanner.Bytes()

		switch {
		// {"resourceSpans"
		case tracesFingerPrint.Match(line):
			if w.tracesConsumer != nil {
				t, err := tracesUnmarshaler.UnmarshalTraces(line)
				if err != nil {
					allErrors = append(allErrors, err)
				} else {
					err := w.tracesConsumer.ConsumeTraces(context.Background(), t)
					allErrors = append(allErrors, err)
				}
			}
		// {"resourceMetrics"
		case metricsFingerPrint.Match(line):
			if w.metricsConsumer != nil {
				l, err := metricsUnmarshaler.UnmarshalMetrics(line)
				if err != nil {
					allErrors = append(allErrors, err)
				} else {
					err := w.metricsConsumer.ConsumeMetrics(context.Background(), l)
					allErrors = append(allErrors, err)
				}
			}
			// {"resourceLogs"
		case logsFingerPrint.Match(line):
			if w.logsConsumer != nil {
				l, err := logsUnmarshaler.UnmarshalLogs(line)
				if err != nil {
					allErrors = append(allErrors, err)
				} else {
					err := w.logsConsumer.ConsumeLogs(context.Background(), l)
					allErrors = append(allErrors, err)
				}
			}
		}
	}

	return multierr.Combine(allErrors...)
}

func (w *watcher) stop() error {
	close(w.done)
	return w.fswatcher.Close()
}
