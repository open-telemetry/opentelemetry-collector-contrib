// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver/internal"

import (
	"context"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.opentelemetry.io/collector/config/configretry"
	"go.uber.org/zap"
)

func NewExponentialBackOff(backOffConfig *configretry.BackOffConfig) *backoff.ExponentialBackOff {
	return &backoff.ExponentialBackOff{
		InitialInterval:     backOffConfig.InitialInterval,
		RandomizationFactor: backOffConfig.RandomizationFactor,
		Multiplier:          backOffConfig.Multiplier,
		MaxInterval:         backOffConfig.MaxInterval,
		MaxElapsedTime:      backOffConfig.MaxElapsedTime,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
}

// Generic function to make an API call with exponential backoff and context cancellation handling.
func MakeAPICallWithRetry[T any](
	ctx context.Context,
	shutdownChan chan struct{},
	logger *zap.Logger,
	apiCall func() (*T, error),
	isThrottlingError func(error) bool,
	backOffConfig *backoff.ExponentialBackOff,
) (*T, error) {
	// Immediately check for context cancellation or server shutdown.
	select {
	case <-ctx.Done():
		return nil, errors.New("request was cancelled or timed out")
	case <-shutdownChan:
		return nil, errors.New("request is cancelled due to server shutdown")
	case <-time.After(50 * time.Millisecond):
	}

	// Make the initial API call.
	resp, err := apiCall()
	if err == nil {
		return resp, nil
	}

	// If the error is not due to request throttling, return the error.
	if !isThrottlingError(err) {
		return nil, err
	}

	// Initialize the backoff mechanism for retrying the API call.
	expBackoff := &backoff.ExponentialBackOff{
		InitialInterval:     backOffConfig.InitialInterval,
		RandomizationFactor: backOffConfig.RandomizationFactor,
		Multiplier:          backOffConfig.Multiplier,
		MaxInterval:         backOffConfig.MaxInterval,
		MaxElapsedTime:      backOffConfig.MaxElapsedTime,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
	expBackoff.Reset()
	attempts := 0

	// Retry loop for handling throttling errors.
	for {
		attempts++
		delay := expBackoff.NextBackOff()
		if delay == backoff.Stop {
			return resp, err
		}
		logger.Warn("server busy, retrying request",
			zap.Int("attempts", attempts),
			zap.Duration("delay", delay))

		// Handle context cancellation or shutdown before retrying.
		select {
		case <-ctx.Done():
			return nil, errors.New("request was cancelled or timed out")
		case <-shutdownChan:
			return nil, errors.New("request is cancelled due to server shutdown")
		case <-time.After(delay):
		}

		// Retry the API call.
		resp, err = apiCall()
		if err == nil {
			return resp, nil
		}
		if !isThrottlingError(err) {
			break
		}
	}

	return nil, err
}
