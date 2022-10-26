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

package filereloadereceiver

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

var errUnusedReceiver = errors.New("receiver defined but not used by any pipeline")

type builtReceivers map[config.ComponentID]component.Receiver

type receiverBuilder struct {
	settings  component.ReceiverCreateSettings
	host      component.Host
	pipelines builtPipelines
	cfg       *runnerConfig
}

func buildReceivers(
	ctx context.Context,
	settings component.TelemetrySettings,
	buildInfo component.BuildInfo,
	host component.Host,
	cfg *runnerConfig,
	builtPipelines builtPipelines,
	cType config.Type,
) (builtReceivers, error) {
	receivers := make(builtReceivers)
	for recvID, recvCfg := range cfg.Receivers {
		set := component.ReceiverCreateSettings{
			TelemetrySettings: component.TelemetrySettings{
				Logger: settings.Logger.With(
					zap.String(ZapKindKey, "receiver"),
					zap.String(ZapNameKey, recvID.String())),
				TracerProvider: settings.TracerProvider,
				MeterProvider:  settings.MeterProvider,
				MetricsLevel:   settings.MetricsLevel,
			},
			BuildInfo: buildInfo,
		}

		rb := &receiverBuilder{
			settings:  set,
			host:      host,
			pipelines: builtPipelines,
			cfg:       cfg,
		}

		rcv, err := rb.buildReceiver(ctx, recvID, recvCfg, cType)
		if err != nil {
			if err == errUnusedReceiver {
				set.Logger.Info("Ignoring receiver as it is not used by any pipeline")
				continue
			}
			return nil, err
		}
		receivers[recvID] = rcv
	}

	return receivers, nil
}

func (rb *receiverBuilder) buildReceiver(
	ctx context.Context,
	id config.ComponentID,
	receiver config.Receiver,
	cType config.Type,
) (component.Receiver, error) {
	factory := rb.host.GetFactory(component.KindReceiver, receiver.ID().Type())
	if factory == nil {
		return nil, fmt.Errorf("unable to lookup factory for receiver %q", receiver.ID().String())
	}

	receiverFactory := factory.(component.ReceiverFactory)
	toAttach, err := rb.pipelinesToAttach(id)
	if err != nil {
		return nil, err
	}

	if len(toAttach) == 0 {
		return nil, fmt.Errorf("no pipelines to attach to for receiver %q", id)
	}

	// This is intentional in the sense that requiring to support this requires support for fanout consumers
	// which are internal packages in core otel collector repository. We can revisit this assumption if we find a valid
	// use case for the same.
	if len(toAttach) > 1 {
		return nil, fmt.Errorf("more than one pipeline to attach to for receiver %q", id)
	}

	switch cType {
	case config.MetricsDataType:
		return receiverFactory.CreateMetricsReceiver(ctx, rb.settings, receiver, toAttach[0].lastConsumer.(consumer.Metrics))
	case config.LogsDataType:
		return receiverFactory.CreateLogsReceiver(ctx, rb.settings, receiver, toAttach[0].lastConsumer.(consumer.Logs))
	default:
		return nil, fmt.Errorf("error creating receiver %q, data type %q is not supported", id, id.Type())
	}

}

// TODO: When we add logs/traces this needs to be enhanced to use map[component]*pipeline
func (rb *receiverBuilder) pipelinesToAttach(receiverID config.ComponentID) ([]*pipeline, error) {
	toAttach := make([]*pipeline, 0)

	for id, cfg := range rb.cfg.PartialPipelines {
		p, ok := rb.pipelines[id]
		if !ok {
			return nil, fmt.Errorf("cant find pipeline %q", id)
		}

		if hasReceiver(cfg, receiverID) {
			toAttach = append(toAttach, p)
		}
	}

	return toAttach, nil
}

func hasReceiver(pipeline *partialPipelineConfig, receiverID config.ComponentID) bool {
	for _, id := range pipeline.Receivers {
		if id == receiverID {
			return true
		}
	}
	return false
}
