// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package consumerretry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/consumerretry"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type logsConsumer struct {
	consumer.Logs
	cfg    Config
	logger *zap.Logger
}

func NewLogs(config Config, logger *zap.Logger, next consumer.Logs) consumer.Logs {
	return &logsConsumer{
		Logs:   next,
		cfg:    config,
		logger: logger,
	}
}

func (lc *logsConsumer) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	if !lc.cfg.Enabled {
		err := lc.Logs.ConsumeLogs(ctx, logs)
		if err != nil {
			lc.logger.Error("ConsumeLogs() failed. "+
				"Enable retry_on_failure to slow down reading logs and avoid dropping.", zap.Error(err))
		}
		return err
	}

	// Do not use NewExponentialBackOff since it calls Reset and the code here must
	// call Reset after changing the InitialInterval (this saves an unnecessary call to Now).
	expBackoff := backoff.ExponentialBackOff{
		MaxElapsedTime:      lc.cfg.MaxElapsedTime,
		InitialInterval:     lc.cfg.InitialInterval,
		MaxInterval:         lc.cfg.MaxInterval,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
	expBackoff.Reset()

	span := trace.SpanFromContext(ctx)
	retryNum := int64(0)
	retryableErr := consumererror.Logs{}
	for {
		span.AddEvent(
			"Sending logs.",
			trace.WithAttributes(attribute.Int64("retry_num", retryNum)))

		err := lc.Logs.ConsumeLogs(ctx, logs)
		if err == nil {
			return nil
		}

		if consumererror.IsPermanent(err) {
			lc.logger.Error(
				"ConsumeLogs() failed. The error is not retryable. Dropping data.",
				zap.Error(err),
				zap.Int("dropped_items", logs.LogRecordCount()),
			)
			return err
		}

		if errors.As(err, &retryableErr) {
			logs = retryableErr.Data()
		}

		// TODO: take delay from the error once it is available in the consumererror package.
		backoffDelay := expBackoff.NextBackOff()
		if backoffDelay == backoff.Stop {
			lc.logger.Error("Max elapsed time expired. Dropping data.", zap.Error(err), zap.Int("dropped_items",
				logs.LogRecordCount()))
			return err
		}

		backoffDelayStr := backoffDelay.String()
		span.AddEvent(
			"ConsumeLogs() failed. Will retry the request after interval.",
			trace.WithAttributes(
				attribute.String("interval", backoffDelayStr),
				attribute.String("error", err.Error())))
		lc.logger.Debug(
			"ConsumeLogs() failed. Will retry the request after interval.",
			zap.Error(err),
			zap.String("interval", backoffDelayStr),
			zap.Int("logs_count", logs.LogRecordCount()),
		)
		retryNum++

		// back-off, but get interrupted when shutting down or request is cancelled or timed out.
		select {
		case <-ctx.Done():
			return fmt.Errorf("context is cancelled or timed out %w", err)
		case <-time.After(backoffDelay):
		}
	}
}
