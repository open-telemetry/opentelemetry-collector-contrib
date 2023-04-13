// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

type BreakerState int

const (
	Open BreakerState = iota
	HalfOpen
	Closed
)

type hecWorker interface {
	send(context.Context, buffer, map[string]string, bool) error
}

type CircuitBreaker struct {
	enabled           bool
	currentState      BreakerState
	timeout           time.Duration
	threshold         uint32
	thresholdDuration time.Duration
	failures          uint32
	lastFailure       time.Time
	breakerOpened     time.Time
	mutex             sync.Mutex
	logger            *zap.Logger
}

func (cb *CircuitBreaker) canExecute(failover bool) bool {

	if !cb.enabled {
		return true
	}

	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	if cb.currentState == Open && !cb.breakerOpened.IsZero() && time.Now().After(cb.breakerOpened.Add(cb.timeout)) {
		cb.logger.Info("Breaker Half Open.", zap.Bool("failover", failover))
		cb.currentState = HalfOpen
	} else if cb.currentState == Closed && !cb.lastFailure.IsZero() && time.Now().After(cb.lastFailure.Add(cb.thresholdDuration)) {
		cb.logger.Info("Reset Last Failure.", zap.Bool("failover", failover))
		cb.failures = 0
		cb.lastFailure = time.Time{}
	}

	return cb.currentState != Open
}

func (cb *CircuitBreaker) updateState(successfulRequest bool, failover bool) {

	if !cb.enabled || cb.threshold == 0 {
		return
	}

	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	if successfulRequest {
		if cb.currentState == HalfOpen {
			cb.logger.Info("Breaker Closed.", zap.Bool("failover", failover))
			cb.currentState = Closed
			cb.breakerOpened = time.Time{}
			cb.lastFailure = time.Time{}
		}
	} else {
		now := time.Now()
		cb.lastFailure = now
		if cb.currentState == HalfOpen {
			cb.logger.Info("Breaker Open.", zap.Uint32("threshold", cb.threshold), zap.Uint32("failures", cb.failures), zap.Bool("failover", failover))
			cb.currentState = Open
			cb.breakerOpened = now
		} else if cb.currentState == Closed {
			cb.failures++
			if cb.failures > cb.threshold {
				cb.logger.Info("Breaker Open.", zap.Uint32("threshold", cb.threshold), zap.Uint32("failures", cb.failures), zap.Bool("failover", failover))
				cb.currentState = Open
				cb.breakerOpened = now
			}
		}
	}
}

type defaultHecWorker struct {
	url             *url.URL
	client          *http.Client
	headers         map[string]string
	logger          *zap.Logger
	primaryBreaker  *CircuitBreaker
	failoverURL     *url.URL
	failoverBreaker *CircuitBreaker
}

func NewDefaultHecWorker(primaryURL *url.URL, failoverURL *url.URL, breakerConfig BreakerSettings, client *http.Client, headers map[string]string, logger *zap.Logger) hecWorker {
	logger.Info("Breaker Settings", zap.Any("bs", breakerConfig))
	return &defaultHecWorker{
		url:         primaryURL,
		failoverURL: failoverURL,
		client:      client,
		headers:     headers,
		logger:      logger,
		primaryBreaker: &CircuitBreaker{
			enabled:           breakerConfig.Enabled,
			currentState:      Closed,
			timeout:           breakerConfig.Timeout,
			threshold:         breakerConfig.FailureThreshold,
			thresholdDuration: breakerConfig.ThresholdDuration,
			failures:          0,
			lastFailure:       time.Time{},
			breakerOpened:     time.Time{},
			logger:            logger,
		},
		failoverBreaker: &CircuitBreaker{
			enabled:           breakerConfig.Enabled,
			currentState:      Closed,
			timeout:           breakerConfig.Timeout,
			threshold:         breakerConfig.FailureThreshold,
			thresholdDuration: breakerConfig.ThresholdDuration,
			failures:          0,
			lastFailure:       time.Time{},
			breakerOpened:     time.Time{},
			logger:            logger,
		},
	}
}

func (hec *defaultHecWorker) send(ctx context.Context, buf buffer, headers map[string]string, failover bool) error {

	if !failover && hec.primaryBreaker != nil && !hec.primaryBreaker.canExecute(false) {
		hec.logger.Info("Primary Breaker Open!")
		return fmt.Errorf("primary breaker open")
	} else if failover && hec.failoverBreaker != nil && !hec.failoverBreaker.canExecute(true) {
		hec.logger.Info("Failover Breaker Open!")
		return fmt.Errorf("failover breaker open")
	}

	reqURL := hec.url.String()
	if failover {
		if hec.failoverURL == nil {
			return nil
		}

		hec.logger.Info("Sending Failover Request")
		reqURL = hec.failoverURL.String()
	} else {
		hec.logger.Info("Sending Request")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, buf)
	if err != nil {
		hec.logger.Info("Get Request Error!", zap.Error(err))
		hec.updateState(false, failover)
		return consumererror.NewPermanent(err)
	}

	req.ContentLength = int64(buf.Len())

	// Set the headers configured for the client
	for k, v := range hec.headers {
		req.Header.Set(k, v)
	}

	// Set extra headers passed by the caller
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if _, ok := buf.(*cancellableGzipWriter); ok {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, respErr := hec.client.Do(req)
	if respErr != nil {
		hec.logger.Info("Response Error!", zap.Error(respErr))
		hec.updateState(false, failover)
		return respErr
	}
	defer resp.Body.Close()

	err = splunk.HandleHTTPCode(resp)
	if err != nil {
		hec.logger.Info("Status Error!", zap.Error((err)))
		hec.updateState(false, failover)
		return err
	}

	// Do not drain the response when 429 or 502 status code is returned.
	// HTTP client will not reuse the same connection unless it is drained.
	// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18281 for more details.
	if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusBadGateway {
		_, errCopy := io.Copy(io.Discard, resp.Body)
		err = multierr.Combine(err, errCopy)
	}

	hec.logger.Info("Successful Request!", zap.Bool("failover", failover))
	hec.updateState(true, failover)
	return err
}

func (hec *defaultHecWorker) updateState(success bool, failover bool) {
	if !failover && hec.primaryBreaker != nil {
		hec.primaryBreaker.updateState(success, failover)
	} else if failover && hec.failoverBreaker != nil {
		hec.failoverBreaker.updateState(success, failover)
	}
}

var _ hecWorker = &defaultHecWorker{}
