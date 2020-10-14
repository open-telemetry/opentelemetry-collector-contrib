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
	"testing"

	"github.com/observiq/stanza/entry"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap/zaptest"
)

func TestStart(t *testing.T) {
	params := component.ReceiverCreateParams{
		Logger: zaptest.NewLogger(t),
	}
	mockConsumer := mockLogsConsumer{}
	receiver, _ := createLogsReceiver(context.Background(), params, createDefaultConfig(), &mockConsumer)

	err := receiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "receiver start failed")

	obsReceiver := receiver.(*stanzareceiver)
	obsReceiver.emitter.logChan <- entry.New()
	receiver.Shutdown(context.Background())
	require.Equal(t, 1, mockConsumer.received, "one log entry expected")
}

func TestHandleStartError(t *testing.T) {
	params := component.ReceiverCreateParams{
		Logger: zaptest.NewLogger(t),
	}
	mockConsumer := mockLogsConsumer{}

	cfg := createDefaultConfig().(*Config)
	cfg.Pipeline = append(cfg.Pipeline, newUnstartableParams())

	receiver, err := createLogsReceiver(context.Background(), params, cfg, &mockConsumer)
	require.NoError(t, err, "receiver should successfully build")

	err = receiver.Start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err, "receiver fails to start under rare circumstances")
}

func TestHandleConsumeError(t *testing.T) {
	params := component.ReceiverCreateParams{
		Logger: zaptest.NewLogger(t),
	}
	mockConsumer := mockLogsRejecter{}
	receiver, _ := createLogsReceiver(context.Background(), params, createDefaultConfig(), &mockConsumer)

	err := receiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "receiver start failed")

	obsReceiver := receiver.(*stanzareceiver)
	obsReceiver.emitter.logChan <- entry.New()
	receiver.Shutdown(context.Background())
	require.Equal(t, 1, mockConsumer.rejected, "one log entry expected")
}
