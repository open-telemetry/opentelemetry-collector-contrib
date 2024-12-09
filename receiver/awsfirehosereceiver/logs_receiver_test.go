// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/unmarshalertest"
)

type logsRecordConsumer struct {
	result plog.Logs
}

var _ consumer.Logs = (*logsRecordConsumer)(nil)

func (rc *logsRecordConsumer) ConsumeLogs(_ context.Context, logs plog.Logs) error {
	rc.result = logs
	return nil
}

func (rc *logsRecordConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestNewLogsReceiver(t *testing.T) {
	testCases := map[string]struct {
		consumer   consumer.Logs
		recordType string
		wantErr    error
	}{
		"WithInvalidRecordType": {
			consumer:   consumertest.NewNop(),
			recordType: "test",
			wantErr:    errUnrecognizedRecordType,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.RecordType = testCase.recordType
			got, err := newLogsReceiver(
				cfg,
				receivertest.NewNopSettings(),
				defaultLogsUnmarshalers(zap.NewNop()),
				testCase.consumer,
			)
			require.Equal(t, testCase.wantErr, err)
			if testCase.wantErr == nil {
				require.NotNil(t, got)
			} else {
				require.Nil(t, got)
			}
		})
	}
}

func TestLogsConsumer(t *testing.T) {
	testErr := errors.New("test error")
	testCases := map[string]struct {
		unmarshalerErr error
		consumerErr    error
		wantStatus     int
		wantErr        error
	}{
		"WithUnmarshalerError": {
			unmarshalerErr: testErr,
			wantStatus:     http.StatusBadRequest,
			wantErr:        testErr,
		},
		"WithConsumerErrorPermanent": {
			consumerErr: consumererror.NewPermanent(testErr),
			wantStatus:  http.StatusBadRequest,
			wantErr:     consumererror.NewPermanent(testErr),
		},
		"WithConsumerError": {
			consumerErr: testErr,
			wantStatus:  http.StatusServiceUnavailable,
			wantErr:     testErr,
		},
		"WithNoError": {
			wantStatus: http.StatusOK,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mc := &logsConsumer{
				unmarshaler: unmarshalertest.NewErrLogs(testCase.unmarshalerErr),
				consumer:    consumertest.NewErr(testCase.consumerErr),
			}
			gotStatus, gotErr := mc.Consume(context.TODO(), "", nil, nil)
			require.Equal(t, testCase.wantStatus, gotStatus)
			require.Equal(t, testCase.wantErr, gotErr)
		})
	}

	t.Run("WithCommonAttributes", func(t *testing.T) {
		base := plog.NewLogs()
		base.ResourceLogs().AppendEmpty()
		rc := logsRecordConsumer{}
		mc := &logsConsumer{
			unmarshaler: unmarshalertest.NewWithLogs(base),
			consumer:    &rc,
		}
		gotStatus, gotErr := mc.Consume(context.TODO(), "", nil, map[string]string{
			"CommonAttributes": "Test",
		})
		require.Equal(t, http.StatusOK, gotStatus)
		require.NoError(t, gotErr)
		gotRms := rc.result.ResourceLogs()
		require.Equal(t, 1, gotRms.Len())
		gotRm := gotRms.At(0)
		require.Equal(t, 1, gotRm.Resource().Attributes().Len())
	})
}
