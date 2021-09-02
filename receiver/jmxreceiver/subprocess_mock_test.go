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

package jmxreceiver

import (
	"bufio"
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver/internal/subprocess"
)

type Mock struct {
	stdout   chan string
	logger   *zap.Logger
	errorStr string
}

var _ SubprocessInt = (*Mock)(nil)

func NewMockSubprocess(logger *zap.Logger, errorStr string) *Mock {
	return &Mock{
		stdout:   make(chan string),
		logger:   logger,
		errorStr: errorStr,
	}
}

func (subprocess *Mock) Stdout() chan string {
	return subprocess.stdout
}

func (subprocess *Mock) Shutdown(context.Context) error {
	time.Sleep(time.Second)
	return nil
}

func (subprocess *Mock) Start(context.Context) error {
	go run(bufio.NewScanner(strings.NewReader(subprocess.errorStr)), subprocess.Stdout(), subprocess.logger)
	return nil
}

func run(stdoutScanner *bufio.Scanner, stdoutChan chan<- string, logger *zap.Logger) {
	subprocess.CollectStdout(stdoutScanner, stdoutChan, logger)
	close(stdoutChan)
}
