// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License. language governing permissions and
// limitations under the License.

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
