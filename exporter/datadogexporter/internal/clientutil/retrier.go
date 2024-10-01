// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/clientutil"

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/scrub"
)

type Retrier struct {
	cfg      configretry.BackOffConfig
	logger   *zap.Logger
	scrubber scrub.Scrubber
}

func NewRetrier(logger *zap.Logger, settings configretry.BackOffConfig, scrubber scrub.Scrubber) *Retrier {
	return &Retrier{
		cfg:      settings,
		logger:   logger,
		scrubber: scrubber,
	}
}

// DoWithRetries does a function with retries. This is a condensed version of the code on
// the exporterhelper, which we reuse here since we want custom retry logic.
func (r *Retrier) DoWithRetries(ctx context.Context, fn func(context.Context) error) (int64, error) {
	if !r.cfg.Enabled {
		return 0, fn(ctx)
	}

	// Do not use NewExponentialBackOff since it calls Reset and the code here must
	// call Reset after changing the InitialInterval (this saves an unnecessary call to Now).
	expBackoff := backoff.ExponentialBackOff{
		InitialInterval:     r.cfg.InitialInterval,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxInterval:         r.cfg.MaxInterval,
		MaxElapsedTime:      r.cfg.MaxElapsedTime,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
	expBackoff.Reset()
	retryNum := int64(0)
	for {
		err := fn(ctx)
		if err == nil {
			return retryNum, nil
		}

		err = r.scrubber.Scrub(err)

		if consumererror.IsPermanent(err) {
			return retryNum, err
		}

		backoffDelay := expBackoff.NextBackOff()
		if backoffDelay == backoff.Stop {
			err = fmt.Errorf("max elapsed time expired %w", err)
			return retryNum, err
		}

		backoffDelayStr := backoffDelay.String()
		r.logger.Debug(
			"Request failed with retriable errors. Will retry the request after interval. (You can safely discard this log if requests eventually go through.)",
			zap.Error(err),
			zap.String("interval", backoffDelayStr),
			zap.Int64("retry attempts", retryNum),
		)
		retryNum++

		// back-off, but get interrupted when shutting down or request is cancelled or timed out.
		select {
		case <-ctx.Done():
			return retryNum, fmt.Errorf("request is cancelled or timed out %w", err)
		case <-time.After(backoffDelay):
		}
	}
}
