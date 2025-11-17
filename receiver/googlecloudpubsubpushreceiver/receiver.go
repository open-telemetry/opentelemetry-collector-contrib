// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubpushreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

type pubSubPushReceiver struct {
	cfg *Config
}

var _ receiver.Logs = (*pubSubPushReceiver)(nil)

func newPubSubPushReceiver(cfg *Config) *pubSubPushReceiver {
	return &pubSubPushReceiver{
		cfg: cfg,
	}
}

func (p *pubSubPushReceiver) Start(_ context.Context, host component.Host) error {
	_, errLoad := loadEncodingExtension[encoding.LogsUnmarshalerExtension](
		host, p.cfg.Encoding, "logs",
	)
	if errLoad != nil {
		return fmt.Errorf("failed to load encoding extension: %w", errLoad)
	}

	return nil
}

func (*pubSubPushReceiver) Shutdown(_ context.Context) error {
	return nil
}

func loadEncodingExtension[T any](host component.Host, encoding component.ID, signal string) (T, error) {
	var zero T
	ext, ok := host.GetExtensions()[encoding]
	if !ok {
		return zero, fmt.Errorf("extension %q not found", encoding.String())
	}
	unmarshaler, ok := ext.(T)
	if !ok {
		return zero, fmt.Errorf("extension %q is not a %s unmarshaler", encoding, signal)
	}
	return unmarshaler, nil
}
