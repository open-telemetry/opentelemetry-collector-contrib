// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubpushreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver/internal/metadata"
)

var receivers = sharedcomponent.NewSharedComponents()

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability),
	)
}

func getOrAddReceiver(
	ctx context.Context,
	cfg component.Config,
	set receiver.Settings,
) (*sharedcomponent.SharedComponent, error) {
	pubSubCfg := cfg.(*Config)
	var err error
	comp := receivers.GetOrAdd(pubSubCfg, func() component.Component {
		var r *pubSubPushReceiver
		r, err = newPubSubPushReceiver(ctx, cfg.(*Config), set)
		if err != nil {
			return nil
		}
		return r
	})
	if err != nil {
		return nil, err
	}
	return comp, nil
}

func createLogsReceiver(
	ctx context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextLogs consumer.Logs,
) (receiver.Logs, error) {
	comp, err := getOrAddReceiver(ctx, cfg, set)
	if err != nil {
		return nil, err
	}
	if r, ok := comp.Unwrap().(*pubSubPushReceiver); ok {
		r.registerLogsConsumer(nextLogs)
		return r, nil
	}
	return nil, errors.New("unexpected: shared component is not a PubSubPushReceiver")
}
