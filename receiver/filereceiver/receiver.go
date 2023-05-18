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

type fileReceiver struct {
	consumer consumer.Metrics
	logger   *zap.Logger
	cancel   context.CancelFunc
	path     string
	throttle float64
}

func (r *fileReceiver) Start(_ context.Context, _ component.Host) error {
	var ctx context.Context
	ctx, r.cancel = context.WithCancel(context.Background())

	file, err := os.Open(r.path)
	if err != nil {
		return fmt.Errorf("failed to open file %q: %w", r.path, err)
	}

	fr := newFileReader(r.consumer, file, newReplayTimer(r.throttle))
	go func() {
		err := fr.readAll(ctx)
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

func (r *fileReceiver) Shutdown(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}
