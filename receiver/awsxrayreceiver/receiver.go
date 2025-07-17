// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/udppoller"
)

const (
	// number of goroutines polling the UDP socket.
	// https://github.com/aws/aws-xray-daemon/blob/master/pkg/cfg/cfg.go#L184
	maxPollerCount = 2
)

// xrayReceiver implements the receiver.Traces interface for converting
// AWS X-Ray segment document into the OT internal trace format.
type xrayReceiver struct {
	poller   udppoller.Poller
	server   proxy.Server
	settings receiver.Settings
	consumer consumer.Traces
	obsrecv  *receiverhelper.ObsReport
	registry telemetry.Registry
}

func newReceiver(config *Config,
	consumer consumer.Traces,
	set receiver.Settings,
) (receiver.Traces, error) {
	set.Logger.Info("Going to listen on endpoint for X-Ray segments",
		zap.String(udppoller.Transport, config.Endpoint))
	poller, err := udppoller.New(&udppoller.Config{
		Transport:          string(config.Transport),
		Endpoint:           config.Endpoint,
		NumOfPollerToStart: maxPollerCount,
	}, set)
	if err != nil {
		return nil, err
	}

	set.Logger.Info("Listening on endpoint for X-Ray segments",
		zap.String(udppoller.Transport, config.Endpoint))

	srv, err := proxy.NewServer(config.ProxyServer, set.Logger)
	if err != nil {
		return nil, err
	}

	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              udppoller.Transport,
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}

	return &xrayReceiver{
		poller:   poller,
		server:   srv,
		settings: set,
		consumer: consumer,
		obsrecv:  obsrecv,
		registry: telemetry.GlobalRegistry(),
	}, nil
}

func (x *xrayReceiver) Start(ctx context.Context, _ component.Host) error {
	// TODO: Might want to pass `host` into read() below to report a fatal error
	x.poller.Start(ctx)
	go x.start()
	go func() {
		_ = x.server.ListenAndServe()
	}()
	x.settings.Logger.Info("X-Ray TCP proxy server started")
	return nil
}

func (x *xrayReceiver) Shutdown(ctx context.Context) error {
	var err error
	if pollerErr := x.poller.Close(); pollerErr != nil {
		err = fmt.Errorf("failed to close poller: %w", pollerErr)
	}

	if proxyErr := x.server.Shutdown(ctx); proxyErr != nil {
		err = errors.Join(err, fmt.Errorf("failed to close proxy: %w", proxyErr))
	}
	return err
}

func (x *xrayReceiver) start() {
	incomingSegments := x.poller.SegmentsChan()
	for seg := range incomingSegments {
		ctx := x.obsrecv.StartTracesOp(seg.Ctx)
		traces, totalSpanCount, err := translator.ToTraces(seg.Payload, x.registry.LoadOrNop(x.settings.ID))
		if err != nil {
			x.settings.Logger.Warn("X-Ray segment to OT traces conversion failed", zap.Error(err))
			x.obsrecv.EndTracesOp(ctx, metadata.Type.String(), totalSpanCount, err)
			continue
		}

		err = x.consumer.ConsumeTraces(ctx, traces)
		if err != nil {
			x.settings.Logger.Warn("Trace consumer errored out", zap.Error(err))
			x.obsrecv.EndTracesOp(ctx, metadata.Type.String(), totalSpanCount, err)
			continue
		}
		x.obsrecv.EndTracesOp(ctx, metadata.Type.String(), totalSpanCount, nil)
	}
}
