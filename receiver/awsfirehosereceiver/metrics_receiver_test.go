// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsfirehosereceiver

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/unmarshalertest"
)

type recordConsumer struct {
	result pmetric.Metrics
}

var _ consumer.Metrics = (*recordConsumer)(nil)

func (rc *recordConsumer) ConsumeMetrics(_ context.Context, metrics pmetric.Metrics) error {
	rc.result = metrics
	return nil
}

func (rc *recordConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestNewMetricsReceiver(t *testing.T) {
	testCases := map[string]struct {
		consumer   consumer.Metrics
		recordType string
		wantErr    error
	}{
		"WithNilConsumer": {
			wantErr: component.ErrNilNextConsumer,
		},
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
			got, err := newMetricsReceiver(
				cfg,
				componenttest.NewNopReceiverCreateSettings(),
				defaultMetricsUnmarshalers(zap.NewNop()),
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

func TestMetricsConsumer(t *testing.T) {
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
		"WithConsumerError": {
			consumerErr: testErr,
			wantStatus:  http.StatusInternalServerError,
			wantErr:     testErr,
		},
		"WithNoError": {
			wantStatus: http.StatusOK,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mc := &metricsConsumer{
				unmarshaler: unmarshalertest.NewErrMetrics(testCase.unmarshalerErr),
				consumer:    consumertest.NewErr(testCase.consumerErr),
			}
			gotStatus, gotErr := mc.Consume(context.TODO(), nil, nil)
			require.Equal(t, testCase.wantStatus, gotStatus)
			require.Equal(t, testCase.wantErr, gotErr)
		})
	}

	t.Run("WithCommonAttributes", func(t *testing.T) {
		base := pmetric.NewMetrics()
		base.ResourceMetrics().AppendEmpty()
		rc := recordConsumer{}
		mc := &metricsConsumer{
			unmarshaler: unmarshalertest.NewWithMetrics(base),
			consumer:    &rc,
		}
		gotStatus, gotErr := mc.Consume(context.TODO(), nil, map[string]string{
			"CommonAttributes": "Test",
		})
		require.Equal(t, http.StatusOK, gotStatus)
		require.NoError(t, gotErr)
		gotRms := rc.result.ResourceMetrics()
		require.Equal(t, 1, gotRms.Len())
		gotRm := gotRms.At(0)
		require.Equal(t, 1, gotRm.Resource().Attributes().Len())
	})
}
