// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package wavefrontreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

var _ receiver.Metrics = (*metricsReceiver)(nil)

type metricsReceiver struct {
	cfg            *Config
	set            receiver.Settings
	nextConsumer   consumer.Metrics
	carbonReceiver receiver.Metrics
}

func newMetricsReceiver(cfg *Config, set receiver.Settings, nextConsumer consumer.Metrics) *metricsReceiver {
	return &metricsReceiver{
		cfg:          cfg,
		set:          set,
		nextConsumer: nextConsumer,
	}
}

func (r *metricsReceiver) Start(ctx context.Context, host component.Host) error {
	fact := carbonreceiver.NewFactory()

	// Wavefront is very similar to Carbon: it is TCP based in which each received
	// text line represents a single metric data point. They differ on the format
	// of their textual representation.
	//
	// The Wavefront receiver leverages the Carbon receiver code by implementing
	// a dedicated parser for its format.
	carbonCfg := &carbonreceiver.Config{
		AddrConfig: confignet.AddrConfig{
			Endpoint:  r.cfg.Endpoint,
			Transport: confignet.TransportTypeTCP,
		},
		TCPIdleTimeout: r.cfg.TCPIdleTimeout,
		Parser: &protocol.Config{
			Type: "plaintext", // TODO: update after other parsers are implemented for Carbon receiver.
			Config: &wavefrontParser{
				ExtractCollectdTags: r.cfg.ExtractCollectdTags,
			},
		},
	}

	set := r.set
	set.ID = component.NewIDWithName(fact.Type(), r.set.ID.String())
	carbonReceiver, err := fact.CreateMetrics(ctx, set, carbonCfg, r.nextConsumer)
	if err != nil {
		return err
	}
	r.carbonReceiver = carbonReceiver

	return r.carbonReceiver.Start(ctx, host)
}

func (r *metricsReceiver) Shutdown(ctx context.Context) error {
	if r.carbonReceiver != nil {
		return r.carbonReceiver.Shutdown(ctx)
	}
	return nil
}
