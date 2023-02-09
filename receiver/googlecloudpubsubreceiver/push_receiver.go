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

package googlecloudpubsubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type pubsubPushReceiver struct {
	*pubsubReceiver
	stopCh chan struct{}
	server *http.Server
}

func (receiver *pubsubPushReceiver) Start(ctx context.Context, host component.Host) error {
	var startErr error
	receiver.tracesUnmarshaler = &ptrace.ProtoUnmarshaler{}
	receiver.metricsUnmarshaler = &pmetric.ProtoUnmarshaler{}
	receiver.logsUnmarshaler = &plog.ProtoUnmarshaler{}
	receiver.startOnce.Do(func() {
		listener, err := receiver.config.Push.ToListener()
		if err != nil {
			startErr = fmt.Errorf("failed to bind to address %s: %w", receiver.config.Push.Endpoint, err)
		} else {
			mux := http.NewServeMux()
			mux.Handle(receiver.config.Push.Path, receiver)
			receiver.server, err = receiver.config.Push.ToServer(host, receiver.telemetrySettings, mux)
			if err != nil {
				startErr = err
			} else {
				receiver.stopCh = make(chan struct{})
				go func() {
					defer close(receiver.stopCh)

					// The listener ownership goes to the server.
					if err = receiver.server.Serve(listener); !errors.Is(err, http.ErrServerClosed) && err != nil {
						host.ReportFatalError(err)
					}
				}()
				receiver.logger.Info("Started Google Pubsub push http handler", zap.String("endpoint", receiver.config.Endpoint), zap.String("path", receiver.config.Push.Path))
			}
		}
	})
	return startErr
}

func (receiver *pubsubPushReceiver) Shutdown(_ context.Context) error {
	if receiver.server == nil {
		return nil
	}
	err := receiver.server.Close()
	if receiver.stopCh != nil {
		<-receiver.stopCh
	}
	receiver.logger.Info("Stopped Google Pubsub push receiver")
	return err
}

type Message struct {
	Attributes  map[string]string `json:"attributes,omitempty"`
	Data        []byte            `json:"data,omitempty"`
	PublishTime string            `json:"messageId"`
	ID          string            `json:"publishTime"`
}

type PubSubMessage struct {
	Message      Message `json:"message"`
	Subscription string  `json:"subscription"`
}

func (receiver *pubsubPushReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var message PubSubMessage
	body, err := io.ReadAll(r.Body)
	if err != nil {
		receiver.logger.Error("Error reading Pubsub message body", zap.Error(err))
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &message); err != nil {
		receiver.logger.Error("Error unmarshalling Pubsub message", zap.Error(err))
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	ctx := context.Background()
	payload := message.Message.Data
	encoding, compression := detectEncoding(receiver.config, message.Message.Attributes)

	switch encoding {
	case otlpProtoTrace:
		if receiver.tracesConsumer != nil {
			err := receiver.handleTrace(ctx, payload, compression)
			receiver.setReturnCode(w, err)
		}
	case otlpProtoMetric:
		if receiver.metricsConsumer != nil {
			err := receiver.handleMetric(ctx, payload, compression)
			receiver.setReturnCode(w, err)
		}
	case otlpProtoLog:
		if receiver.logsConsumer != nil {
			err := receiver.handleLog(ctx, payload, compression)
			receiver.setReturnCode(w, err)
		}
	case rawTextLog:
		if receiver.logsConsumer != nil {
			err := receiver.handleTextPush(ctx, message)
			receiver.setReturnCode(w, err)
		}
	default:
		http.Error(w, "Bad Request", http.StatusBadRequest)
	}
}

func (receiver *pubsubReceiver) setReturnCode(w http.ResponseWriter, err error) {
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
	} else {
		w.WriteHeader(http.StatusAccepted)
	}
}

func (receiver *pubsubReceiver) handleTextPush(ctx context.Context, message PubSubMessage) error {
	ts, err := time.Parse("2006-01-02T15:04:05.000Z", message.Message.PublishTime)
	if err != nil {
		return err
	}
	return receiver.handleLogStrings(ctx, message.Message.Data, ts)
}
