// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package consumerretry

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestConsumeLogs(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		consumer    *MockLogsRejecter
		expectedErr error
	}{
		{
			name:        "no_retry_success",
			expectedErr: nil,
			cfg:         NewDefaultConfig(),
			consumer:    NewMockLogsRejecter(0),
		},
		{
			name:        "permanent_error",
			expectedErr: consumererror.NewPermanent(errors.New("permanent error")),
			cfg:         Config{Enabled: true},
			consumer:    NewMockLogsRejecter(-1),
		},
		{
			name:        "timeout_error",
			expectedErr: errors.New("retry later"),
			cfg: Config{
				Enabled:         true,
				InitialInterval: 1 * time.Millisecond,
				MaxInterval:     5 * time.Millisecond,
				MaxElapsedTime:  10 * time.Millisecond,
			},
			consumer: NewMockLogsRejecter(20),
		},
		{
			name:        "retry_success",
			expectedErr: nil,
			cfg: Config{
				Enabled:         true,
				InitialInterval: 1 * time.Millisecond,
				MaxInterval:     2 * time.Millisecond,
				MaxElapsedTime:  100 * time.Millisecond,
			},
			consumer: NewMockLogsRejecter(5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumer := NewLogs(tt.cfg, zap.NewNop(), tt.consumer)
			err := consumer.ConsumeLogs(context.Background(), testdata.GenerateLogsTwoLogRecordsSameResource())
			assert.Equal(t, tt.expectedErr, err)
			if err == nil {
				assert.Len(t, tt.consumer.AllLogs(), 1)
				assert.Equal(t, 2, tt.consumer.AllLogs()[0].LogRecordCount())
				if tt.consumer.acceptAfter > 0 {
					assert.Equal(t, tt.consumer.rejectCount.Load(), tt.consumer.acceptAfter)
				}
			} else if tt.consumer.acceptAfter > 0 {
				assert.Less(t, tt.consumer.rejectCount.Load(), tt.consumer.acceptAfter)
			}
		})
	}
}

func TestConsumeLogs_ContextDeadline(t *testing.T) {
	consumer := NewLogs(Config{
		Enabled:         true,
		InitialInterval: 1 * time.Millisecond,
		MaxInterval:     5 * time.Millisecond,
		MaxElapsedTime:  50 * time.Millisecond,
	}, zap.NewNop(), NewMockLogsRejecter(10))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	err := consumer.ConsumeLogs(ctx, testdata.GenerateLogsTwoLogRecordsSameResource())
	assert.ErrorContains(t, err, "context is cancelled or timed out retry later")
}

func TestConsumeLogs_PartialRetry(t *testing.T) {
	sink := &mockPartialLogsRejecter{}
	consumer := NewLogs(Config{
		Enabled:         true,
		InitialInterval: 1 * time.Millisecond,
		MaxInterval:     5 * time.Millisecond,
		MaxElapsedTime:  50 * time.Millisecond,
	}, zap.NewNop(), sink)

	logs := testdata.GenerateLogsTwoLogRecordsSameResource()
	testdata.GenerateLogsOneLogRecordNoResource().ResourceLogs().MoveAndAppendTo(logs.ResourceLogs())
	assert.NoError(t, consumer.ConsumeLogs(context.Background(), logs))

	// Verify the logs batch is broken into two parts, one with the partial error and one without.
	assert.Len(t, sink.AllLogs(), 2)
	assert.Equal(t, 1, sink.AllLogs()[0].ResourceLogs().Len())
	assert.Equal(t, 2, sink.AllLogs()[0].LogRecordCount())
	assert.Equal(t, 1, sink.AllLogs()[1].ResourceLogs().Len())
	assert.Equal(t, 1, sink.AllLogs()[1].LogRecordCount())
}
