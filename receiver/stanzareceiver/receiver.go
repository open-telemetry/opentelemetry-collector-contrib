// Copyright 2019, OpenTelemetry Authors
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

package stanzareceiver

import (
	"context"

	stanza "github.com/observiq/stanza/agent"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type stanzareceiver struct {
	agent    *stanza.LogAgent
	emitter  *LogEmitter
	consumer consumer.LogsConsumer
	logger   *zap.Logger
}

// Ensure this factory adheres to required interface
var _ component.LogsReceiver = (*stanzareceiver)(nil)

// Start tells the receiver to start
func (r *stanzareceiver) Start(ctx context.Context, host component.Host) error {
	// TODO
	return nil
}

// Shutdown is invoked during service shutdown
func (r *stanzareceiver) Shutdown(context.Context) error {
	// TODO
	return nil
}
