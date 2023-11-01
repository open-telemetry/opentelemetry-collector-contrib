// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sflowreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func TestTranslator_SflowToOtelLogs(t *testing.T) {
	type fields struct {
		Logger *zap.Logger
	}
	type args struct {
		sflowData *SFlowData
		config    *Config
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   plog.Logs
	}{
		{
			name:   "Test SflowToOtelLogs",
			fields: fields{Logger: zap.NewNop()},
			args: args{
				sflowData: &SFlowData{
					Version:        1,
					AgentIP:        "127.0.0.1",
					SubAgentID:     1,
					SequenceNumber: 1,
					AgentUptime:    1,
					SampleCount:    1,
					SFlowSample:    []SflowSample{},
				},
				config: &Config{
					Labels: map[string]string{
						"label1": "value1",
					},
				},
			},
			want: plog.Logs{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &Translator{
				Logger: tt.fields.Logger,
			}
			got := tr.SflowToOtelLogs(tt.args.sflowData, tt.args.config)
			assert.NotNil(t, got)
			assert.Equal(t, 1, got.LogRecordCount())
			assert.Equal(t, map[string]any{
				"agentIP":        "127.0.0.1",
				"agentUpTime":    float64(1),
				"label1":         string("value1"),
				"sampleCount":    float64(1),
				"sequenceNumber": float64(1),
				"subAgentID":     float64(1),
				"version":        float64(1),
			}, got.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().AsRaw())
		})
	}
}
