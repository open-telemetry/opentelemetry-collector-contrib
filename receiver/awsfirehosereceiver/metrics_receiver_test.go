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
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
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
				receivertest.NewNopSettings(),
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

func TestSanitizeValue(t *testing.T) {
	testCases := map[string]struct {
		input    string
		expected string
	}{
		"EmptyString": {
			input:    "",
			expected: "",
		},
		"OnlyAlphanumeric": {
			input:    "abc123",
			expected: "abc123",
		},
		"WithUnderscore": {
			input:    "abc_123",
			expected: "abc_123",
		},
		"WithDash": {
			input:    "abc-123",
			expected: "abc-123",
		},
		"WithDot": {
			input:    "abc.123",
			expected: "abc_123",
		},
		"WithSpecialCharacters": {
			input:    "abc!@#$%^&*()",
			expected: "abc",
		},
		"WithMixedCharacters": {
			input:    "abc_123!@#$%^&*()",
			expected: "abc_123",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got := sanitizeValue(testCase.input)
			require.Equal(t, testCase.expected, got)
		})
	}
}

func TestMakePrefixedName(t *testing.T) {
	testCases := map[string]struct {
		namePrefixes []NamePrefixConfig
		attributes   map[string]string
		basename     string
		expected     string
	}{
		"EmptyNamePrefixes": {
			namePrefixes: nil,
			attributes: map[string]string{
				"attr1": "value1",
			},
			basename: "alice.bob",
			expected: "alice.bob",
		},
		"EmptyAttributes": {
			namePrefixes: []NamePrefixConfig{
				{
					AttributeName: "attr1",
					Default:       "default",
				},
			},
			attributes: nil,
			basename:   "alice.bob",
			expected:   "default.alice.bob",
		},
		"EmptyAttributeValue": {
			namePrefixes: []NamePrefixConfig{
				{
					AttributeName: "attr1",
					Default:       "default",
				},
			},
			attributes: map[string]string{
				"attr1": "",
			},
			basename: "alice.bob",
			expected: "default.alice.bob",
		},
		"MissingAttribute": {
			namePrefixes: []NamePrefixConfig{
				{
					AttributeName: "attr1",
					Default:       "default",
				},
			},
			attributes: map[string]string{
				"attr2": "value2",
			},
			basename: "alice.bob",
			expected: "default.alice.bob",
		},
		"Valid": {
			namePrefixes: []NamePrefixConfig{
				{
					AttributeName: "attr1",
					Default:       "default",
				},
			},
			attributes: map[string]string{
				"attr1": "value1",
			},
			basename: "alice.bob",
			expected: "value1.alice.bob",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			ra := pcommon.NewMap()
			for k, v := range testCase.attributes {
				ra.PutStr(k, v)
			}
			got := makePrefixedName(testCase.namePrefixes, testCase.basename, ra)
			require.Equal(t, testCase.expected, got)
		})
	}
}

func TestApplyNamePrefixes(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("attr1", "value1")

	namePrefixes := []NamePrefixConfig{
		{
			AttributeName: "attr1",
			Default:       "default",
		},
	}

	ilm := rm.ScopeMetrics().AppendEmpty()
	m := ilm.Metrics().AppendEmpty()
	m.SetName("name")

	applyNamePrefixes(md, namePrefixes)

	got := m.Name()
	require.Equal(t, "value1.name", got)
}
