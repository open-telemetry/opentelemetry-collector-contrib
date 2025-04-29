// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestMakeAPICallWithRetrySuccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	apiCall := func() (*string, error) {
		result := "success"
		return &result, nil
	}
	isThrottlingError := func(_ error) bool {
		return false
	}

	resp, err := MakeAPICallWithRetry(context.TODO(), make(chan struct{}), logger, apiCall, isThrottlingError, backoff.NewExponentialBackOff())

	assert.NoError(t, err)
	assert.Equal(t, "success", *resp)
}

func TestMakeAPICallWithRetryImmediateFailure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	apiCall := func() (*string, error) {
		return nil, errors.New("some error")
	}
	isThrottlingError := func(_ error) bool {
		return false
	}

	resp, err := MakeAPICallWithRetry(context.TODO(), make(chan struct{}), logger, apiCall, isThrottlingError, backoff.NewExponentialBackOff())

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, "some error", err.Error())
}

func TestMakeAPICallWithRetryThrottlingWithSuccess(t *testing.T) {
	logger := zaptest.NewLogger(t)
	callCount := 0
	apiCall := func() (*string, error) {
		callCount++
		if callCount == 3 {
			result := "success"
			return &result, nil
		}
		return nil, errors.New("throttling error")
	}
	isThrottlingError := func(err error) bool {
		return err.Error() == "throttling error"
	}

	backOffConfig := backoff.NewExponentialBackOff()
	backOffConfig.InitialInterval = 10 * time.Millisecond

	resp, err := MakeAPICallWithRetry(context.TODO(), make(chan struct{}), logger, apiCall, isThrottlingError, backOffConfig)

	assert.NoError(t, err)
	assert.Equal(t, "success", *resp)
	assert.Equal(t, 3, callCount)
}

func TestMakeAPICallWithRetryThrottlingMaxRetries(t *testing.T) {
	logger := zaptest.NewLogger(t)
	apiCall := func() (*string, error) {
		return nil, errors.New("throttling error")
	}
	isThrottlingError := func(err error) bool {
		return err.Error() == "throttling error"
	}

	backOffConfig := backoff.NewExponentialBackOff()
	backOffConfig.MaxElapsedTime = 50 * time.Millisecond

	resp, err := MakeAPICallWithRetry(context.TODO(), make(chan struct{}), logger, apiCall, isThrottlingError, backOffConfig)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, "throttling error", err.Error())
}

func TestMakeAPICallWithRetryContextCancellation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx, cancel := context.WithCancel(context.TODO())
	time.AfterFunc(time.Second, cancel)

	apiCall := func() (*string, error) {
		return nil, errors.New("throttling error")
	}
	isThrottlingError := func(err error) bool {
		return err.Error() == "throttling error"
	}

	resp, err := MakeAPICallWithRetry(ctx, make(chan struct{}), logger, apiCall, isThrottlingError, backoff.NewExponentialBackOff())

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, "request was cancelled or timed out", err.Error())
}

func TestMakeAPICallWithRetryServerShutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	shutdownChan := make(chan struct{})
	time.AfterFunc(time.Second, func() { close(shutdownChan) })

	apiCall := func() (*string, error) {
		return nil, errors.New("throttling error")
	}
	isThrottlingError := func(err error) bool {
		return err.Error() == "throttling error"
	}

	resp, err := MakeAPICallWithRetry(context.TODO(), shutdownChan, logger, apiCall, isThrottlingError, backoff.NewExponentialBackOff())

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, "request is cancelled due to server shutdown", err.Error())
}
