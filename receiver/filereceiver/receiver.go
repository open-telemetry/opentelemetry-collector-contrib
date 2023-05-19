// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filereceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type consumerType struct {
	metricsConsumer consumer.Metrics
	tracesConsumer  consumer.Traces
	logsConsumer    consumer.Logs
}
type fileReceiver struct {
	consumer    consumerType
	logger      *zap.Logger
	cancel      context.CancelFunc
	path        string
	throttle    float64
	format      string
	compression string
}

func (r *fileReceiver) Start(ctx context.Context, _ component.Host) error {
	ctx, r.cancel = context.WithCancel(ctx)

	file, err := os.Open(r.path)
	if err != nil {
		return fmt.Errorf("failed to open file %q: %w", r.path, err)
	}

	fr := newFileReader(r.consumer, file, newReplayTimer(r.throttle), r.format, r.compression)
	go func() {
		var err error
		if r.format == formatTypeProto {
			err = fr.readAllChunks(ctx)
		} else {
			err = fr.readAllLines(ctx)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				r.logger.Debug("EOF reached")
			} else {
				r.logger.Error("failed to read input file", zap.Error(err))
			}
		}
	}()
	return nil
}

func (r *fileReceiver) Shutdown(_ context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}
