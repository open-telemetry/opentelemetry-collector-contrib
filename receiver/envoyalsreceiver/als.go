// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package envoyalsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/envoyalsreceiver"

import (
	"context"
	"errors"
	"fmt"

	alsv3 "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/envoyalsreceiver/internal/als"
)

type alsReceiver struct {
	conf         *Config
	nextConsumer consumer.Logs
	settings     receiver.Settings
	serverGRPC   *grpc.Server

	obsrepGRPC *receiverhelper.ObsReport
}

func (r *alsReceiver) Start(ctx context.Context, host component.Host) error {
	var err error
	r.serverGRPC, err = r.conf.ToServer(ctx, host, r.settings.TelemetrySettings)
	if err != nil {
		return fmt.Errorf("failed create grpc server error: %w", err)
	}

	alsv3.RegisterAccessLogServiceServer(r.serverGRPC, als.New(r.nextConsumer, r.obsrepGRPC))

	err = r.startGRPCServer(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to start grpc server error: %w", err)
	}

	return err
}

func (r *alsReceiver) startGRPCServer(ctx context.Context, host component.Host) error {
	r.settings.Logger.Info("Starting GRPC server", zap.String("endpoint", r.conf.NetAddr.Endpoint))
	listener, err := r.conf.NetAddr.Listen(ctx)
	if err != nil {
		return err
	}

	go func() {
		if errGRPC := r.serverGRPC.Serve(listener); !errors.Is(errGRPC, grpc.ErrServerStopped) && errGRPC != nil {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errGRPC))
		}
	}()
	return nil
}

func (r *alsReceiver) Shutdown(_ context.Context) error {
	if r.serverGRPC != nil {
		r.serverGRPC.GracefulStop()
	}

	return nil
}

func newALSReceiver(cfg *Config, nextConsumer consumer.Logs, settings receiver.Settings) (*alsReceiver, error) {
	r := &alsReceiver{
		conf:         cfg,
		nextConsumer: nextConsumer,
		settings:     settings,
	}

	var err error
	r.obsrepGRPC, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              "grpc",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	return r, nil
}
