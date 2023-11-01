// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sflowreceiver

import (
	"context"
	"reflect"
	"testing"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

func Test_createLogsReceiver(t *testing.T) {
	type args struct {
		ctx          context.Context
		params       receiver.CreateSettings
		basecfg      component.Config
		nextConsumer consumer.Logs
	}
	tests := []struct {
		name    string
		args    args
		want    receiver.Logs
		wantErr bool
	}{
		{
			name: "nil context",
			args: args{
				ctx:          nil,
				params:       receiver.CreateSettings{},
				basecfg:      createDefaultConfig(),
				nextConsumer: nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createLogsReceiver(tt.args.ctx, tt.args.params, tt.args.basecfg, tt.args.nextConsumer)
			if (err != nil) != tt.wantErr {
				t.Errorf("createLogsReceiver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createLogsReceiver() = %v, want %v", got, tt.want)
			}
		})
	}
}
