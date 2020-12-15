// Copyright 2020, OpenTelemetry Authors
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

package stdinreceiver

import (
	"bufio"
	"context"
	"errors"
	"os"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
)

const (
	transport = "stdin"
	format    = "string"
)

var (
	errNilNextLogsConsumer = errors.New("nil logsConsumer")
	listenerEnabled        = sync.Once{}
	listeners              []chan string
	stdin                  = os.Stdin
)

// stdinReceiver implements the component.MetricsReceiver for stdin metric protocol.
type stdinReceiver struct {
	logger       *zap.Logger
	config       *Config
	logsConsumer consumer.LogsConsumer
	shutdown     chan struct{}
}

// NewLogsReceiver creates the stdin receiver with the given configuration.
func NewLogsReceiver(
	logger *zap.Logger,
	config Config,
	nextConsumer consumer.LogsConsumer,
) (component.LogsReceiver, error) {
	if nextConsumer == nil {
		return nil, errNilNextLogsConsumer
	}

	r := &stdinReceiver{
		logger:       logger,
		config:       &config,
		logsConsumer: nextConsumer,
	}

	return r, nil
}

func startStdinListener() {
	listenerEnabled.Do(func() {
		reader := bufio.NewReader(stdin)
		data := make([]byte, 4096)
		for {
			amount, err := reader.Read(data)
			if err != nil {
				return // EOF signal
			}
			if amount == 0 {
				continue
			}
			raw := string(data[:amount])
			splitLines := strings.Split(raw, "\r\n")
			for _, splitLine := range splitLines {
				lines := strings.Split(splitLine, "\n")
				for _, line := range lines {
					for _, listener := range listeners {
						listener <- line
					}
				}
			}
		}
	})
}

// Start starts the stdin receiver, adding it to the stdin listener.
func (r *stdinReceiver) Start(_ context.Context, host component.Host) error {
	//start listening to stdin - do this at most once.

	r.shutdown = make(chan struct{})

	listener := make(chan string)
	listeners = append(listeners, listener)

	go func() {
		go startStdinListener()
		for {
			select {
			case nextLine := <-listener:
				r.consumeLine(nextLine)
			case <-r.shutdown:
				return
			}
		}
	}()

	return nil
}

func (r *stdinReceiver) consumeLine(line string) {
	ctx := obsreport.ReceiverContext(context.Background(), r.config.Name(), transport)
	ctx = obsreport.StartLogsReceiveOp(ctx, r.config.Name(), transport)
	ld := pdata.NewLogs()
	rl := pdata.NewResourceLogs()
	ld.ResourceLogs().Append(rl)
	ill := pdata.NewInstrumentationLibraryLogs()
	rl.InstrumentationLibraryLogs().Append(ill)
	lr := pdata.NewLogRecord()
	ill.Logs().Append(lr)
	lr.Body().SetStringVal(line)
	err := r.logsConsumer.ConsumeLogs(ctx, ld)
	obsreport.EndLogsReceiveOp(ctx, format, 1, err)
}

// Shutdown shuts down the stdin receiver, closing its listener to the stdin loop.
func (r *stdinReceiver) Shutdown(context.Context) error {
	close(r.shutdown)

	return nil
}
