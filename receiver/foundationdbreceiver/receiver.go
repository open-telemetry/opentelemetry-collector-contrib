// Copyright 2022 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package foundationdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/foundationdbreceiver"

import (
	"context"
	"errors"
	"net"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type foundationDBReceiver struct {
	config   *Config
	listener fdbTraceListener
	consumer consumer.Traces
	logger   *zap.Logger
	handler  fdbTraceHandler
}

// Start the foundationDBReceiver.
func (f *foundationDBReceiver) Start(ctx context.Context, host component.Host) error {
	go func() {
		f.logger.Info("Starting FoundationDB UDP Trace listener server.")
		if err := f.listener.ListenAndServe(f.handler, f.config.MaxPacketSize); err != nil {
			f.logger.Info("FoundationDB UDP Trace listener server stopped.")
			if !errors.Is(err, net.ErrClosed) {
				host.ReportFatalError(err)
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
