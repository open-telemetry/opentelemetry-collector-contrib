// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/unmarshalertest"
)

type logsRecordConsumer struct {
	results []plog.Logs
}

var _ consumer.Logs = (*logsRecordConsumer)(nil)

func (rc *logsRecordConsumer) ConsumeLogs(_ context.Context, logs plog.Logs) error {
	rc.results = append(rc.results, logs)
	return nil
}

func (rc *logsRecordConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestLogsReceiver_Start(t *testing.T) {
	testCases := map[string]struct {
		encoding            string
		recordType          string
		wantUnmarshalerType plog.Unmarshaler
		wantErr             string
	}{
		"WithDefaultEncoding": {
			wantUnmarshalerType: &cwlog.Unmarshaler{},
		},
		"WithBuiltinEncoding": {
			encoding:            "cwlogs",
			wantUnmarshalerType: &cwlog.Unmarshaler{},
		},
		"WithExtensionEncoding": {
			encoding:            "otlp_logs",
			wantUnmarshalerType: plogUnmarshalerExtension{},
		},
		"WithExtensionEncodingNamed": {
			encoding:            "otlp_logs/name",
			wantUnmarshalerType: plogUnmarshalerExtension{},
		},
		"WithDeprecatedRecordType": {
			recordType:          "otlp_logs",
			wantUnmarshalerType: plogUnmarshalerExtension{},
		},
		"WithUnknownEncoding": {
			encoding: "invalid",
			wantErr:  "failed to start consumer: failed to load encoding extension: unknown encoding extension \"invalid\"",
		},
		"WithNonLogUnmarshalerExtension": {
			encoding: "otlp_metrics",
			wantErr:  `failed to start consumer: failed to load encoding extension: extension "otlp_metrics" is not a logs unmarshaler`,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Encoding = testCase.encoding
			cfg.RecordType = testCase.recordType
			got, err := newLogsReceiver(
				cfg,
				receivertest.NewNopSettings(metadata.Type),
				consumertest.NewNop(),
			)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.IsType(t, &firehoseReceiver{}, got)
			t.Cleanup(func() {
				require.NoError(t, got.Shutdown(context.Background()))
			})

			host := hostWithExtensions{
				extensions: map[component.ID]component.Component{
					component.MustNewID("otlp_logs"):                 plogUnmarshalerExtension{},
					component.MustNewIDWithName("otlp_logs", "name"): plogUnmarshalerExtension{},
					component.MustNewID("otlp_metrics"):              pmetricUnmarshalerExtension{},
				},
			}

			err = got.Start(context.Background(), host)
			if testCase.wantErr != "" {
				require.EqualError(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.IsType(t,
				testCase.wantUnmarshalerType,
				got.(*firehoseReceiver).consumer.(*logsConsumer).unmarshaler,
			)
		})
	}
}

func TestLogsConsumer_Errors(t *testing.T) {
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
			lc := &logsConsumer{
				unmarshaler: unmarshalertest.NewErrLogs(testCase.unmarshalerErr),
				consumer:    consumertest.NewErr(testCase.consumerErr),
			}
			gotStatus, gotErr := lc.Consume(context.TODO(), newNextRecordFunc([][]byte{{}}), nil)
			require.Equal(t, testCase.wantStatus, gotStatus)
			require.Equal(t, testCase.wantErr, gotErr)
		})
	}
}

func TestLogsConsumer(t *testing.T) {
	t.Run("WithCommonAttributes", func(t *testing.T) {
		base := plog.NewLogs()
		base.ResourceLogs().AppendEmpty()
		rc := logsRecordConsumer{}
		lc := &logsConsumer{
			unmarshaler: unmarshalertest.NewWithLogs(base),
			consumer:    &rc,
		}
		gotStatus, gotErr := lc.Consume(context.TODO(), newNextRecordFunc([][]byte{{}}), map[string]string{
			"CommonAttributes": "Test",
		})
		require.Equal(t, http.StatusOK, gotStatus)
		require.NoError(t, gotErr)
		require.Len(t, rc.results, 1)
		gotRms := rc.results[0].ResourceLogs()
		require.Equal(t, 1, gotRms.Len())
		gotRm := gotRms.At(0)
		require.Equal(t, 1, gotRm.Resource().Attributes().Len())
	})
	t.Run("WithMultipleRecords", func(t *testing.T) {
		logs0, logRecords0 := newLogs("service0", "scope0")
		logRecords0.AppendEmpty().Body().SetStr("record0")
		logRecords0.AppendEmpty().Body().SetStr("record1")

		logs1, logRecords1 := newLogs("service0", "scope0")
		logRecords1.AppendEmpty().Body().SetStr("record2")
		logRecords1.AppendEmpty().Body().SetStr("record3")

		logsRemaining := []plog.Logs{logs0, logs1}
		var unmarshaler unmarshalLogsFunc = func([]byte) (plog.Logs, error) {
			logs := logsRemaining[0]
			logsRemaining = logsRemaining[1:]
			return logs, nil
		}

		rc := logsRecordConsumer{}
		lc := &logsConsumer{unmarshaler: unmarshaler, consumer: &rc}
		nextRecord := newNextRecordFunc(make([][]byte, len(logsRemaining)))
		gotStatus, gotErr := lc.Consume(context.Background(), nextRecord, nil)
		require.Equal(t, http.StatusOK, gotStatus)
		require.NoError(t, gotErr)
		require.Len(t, rc.results, 2)
		assert.NoError(t, plogtest.CompareLogs(logs0, rc.results[0]))
		assert.NoError(t, plogtest.CompareLogs(logs1, rc.results[1]))
	})
}

func newLogs(serviceName, scopeName string) (plog.Logs, plog.LogRecordSlice) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	newResource(serviceName).MoveTo(resourceLogs.Resource())
	newScope(scopeName).MoveTo(scopeLogs.Scope())
	return logs, scopeLogs.LogRecords()
}

func newResource(serviceName string) pcommon.Resource {
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", serviceName)
	return r
}

func newScope(scopeName string) pcommon.InstrumentationScope {
	s := pcommon.NewInstrumentationScope()
	s.SetName(scopeName)
	return s
}

type unmarshalLogsFunc func([]byte) (plog.Logs, error)

func (f unmarshalLogsFunc) UnmarshalLogs(data []byte) (plog.Logs, error) {
	return f(data)
}
