// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package foundationdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/foundationdbreceiver"

import (
	"context"
	"errors"
	"net"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type foundationDBReceiver struct {
	config                 *Config
	listener               fdbTraceListener
	consumer               consumer.Traces
	logger                 *zap.Logger
	receiverCreateSettings receiver.CreateSettings
	handler                fdbTraceHandler
}

// Start the foundationDBReceiver.
func (f *foundationDBReceiver) Start(ctx context.Context, host component.Host) error {
	go func() {
		f.logger.Info("Starting FoundationDB UDP Trace listener server.")
		if err := f.listener.ListenAndServe(f.handler, f.config.MaxPacketSize); err != nil {
			f.logger.Info("FoundationDB UDP Trace listener server stopped.")
			if !errors.Is(err, net.ErrClosed) {
				f.receiverCreateSettings.ReportStatus(component.NewFatalErrorEvent(err))
			}
		}
	}()

	// In practice we have not seen the context canceled, however implementing for correctness.
	go func() {
		for {
			select {
			case <-ctx.Done():
				f.logger.Info("FoundationDB receiver selected Done signal.")
				f.listener.Close()
				return
			}
		}
	}()
	return nil
}

// Shutdown the foundationDBReceiver receiver.
func (f *foundationDBReceiver) Shutdown(context.Context) error {
	f.logger.Info("FoundationDB Trace receiver in Shutdown.")
	err := f.listener.Close()
	if err != nil {
		f.logger.Sugar().Debugf("Error received attempting to close server: %s", err.Error())
	}
	return err
}
