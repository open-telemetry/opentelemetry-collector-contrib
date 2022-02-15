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

package stanza

import (
	"context"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-log-collection/logger"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("stanza_input", func() operator.Builder { return NewInputConfig("") })
}

// NewInputConfig creates a new stanza input config with default values
func NewInputConfig(operatorID string) *InputConfig {
	return &InputConfig{
		InputConfig: helper.NewInputConfig(operatorID, "stanza_input"),
		BufferSize:  100,
	}
}

// InputConfig is the configuration of a stanza input operator.
type InputConfig struct {
	helper.InputConfig `yaml:",inline"`
	BufferSize         int `json:"buffer_size" yaml:"buffer_size"`
}

// Build will build a stanza input operator.
func (c *InputConfig) Build(context operator.BuildContext) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(context)
	if err != nil {
		return nil, err
	}

	receiver := make(logger.Receiver, c.BufferSize)
	context.Logger.AddReceiver(receiver)

	return &Input{
		InputOperator: inputOperator,
		receiver:      receiver,
	}, nil
}

// Input is an operator that receives internal stanza logs.
type Input struct {
	helper.InputOperator

	receiver logger.Receiver
	wg       sync.WaitGroup
	cancel   context.CancelFunc
}

// Start will start reading incoming stanza logs.
func (i *Input) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	i.cancel = cancel
	i.startReading(ctx)
	return nil
}

// Stop will stop reading logs.
func (i *Input) Stop() error {
	i.cancel()
	i.wg.Wait()
	return nil
}

// startReading will start reading stanza logs from the receiver.
func (i *Input) startReading(ctx context.Context) {
	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		for {
			select {
			case <-ctx.Done():
				i.drain(ctx)
				return
			case e := <-i.receiver:
				i.Write(ctx, &e)
			}
		}
	}()
}

// drain will read stanza logs until the receiver is empty.
func (i *Input) drain(ctx context.Context) {
	timeout := time.After(time.Millisecond * 100)
	for {
		select {
		case e := <-i.receiver:
			i.Write(ctx, &e)
		case <-timeout:
			return
		}
	}
}
