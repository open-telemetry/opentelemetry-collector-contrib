// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/clientutil"

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configretry"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/scrub"
)

func TestDoWithRetries(t *testing.T) {
	scrubber := scrub.NewScrubber()
	retrier := NewRetrier(zap.NewNop(), configretry.NewDefaultBackOffConfig(), scrubber)
	ctx := context.Background()

	retryNum, err := retrier.DoWithRetries(ctx, func(context.Context) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, int64(0), retryNum)

	retrier = NewRetrier(zap.NewNop(),
		configretry.BackOffConfig{
			Enabled:         true,
			InitialInterval: 5 * time.Millisecond,
			MaxInterval:     30 * time.Millisecond,
			MaxElapsedTime:  100 * time.Millisecond,
		},
		scrubber,
	)
	retryNum, err = retrier.DoWithRetries(ctx, func(context.Context) error { return errors.New("action failed") })
	require.Error(t, err)
	assert.Positive(t, retryNum)
}

func TestNoRetriesOnPermanentError(t *testing.T) {
	scrubber := scrub.NewScrubber()
	retrier := NewRetrier(zap.NewNop(), configretry.NewDefaultBackOffConfig(), scrubber)
	ctx := context.Background()
	respNonRetriable := http.Response{StatusCode: http.StatusNotFound}

	retryNum, err := retrier.DoWithRetries(ctx, func(context.Context) error {
		return WrapError(errors.New("test"), &respNonRetriable)
	})
	require.Error(t, err)
	assert.Equal(t, int64(0), retryNum)
}
