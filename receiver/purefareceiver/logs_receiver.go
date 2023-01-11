// Copyright 2022 The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package purefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var _ receiver.Logs = (*purefaLogsReceiver)(nil)

type purefaLogsReceiver struct {
	cfg  *Config
	set  receiver.CreateSettings
	next consumer.Logs

	wrapped receiver.Logs
}

func newLogsReceiver(cfg *Config, set receiver.CreateSettings, next consumer.Logs) *purefaLogsReceiver {
	return &purefaLogsReceiver{
		cfg:  cfg,
		set:  set,
		next: next,
	}
}

func (r *purefaLogsReceiver) Start(ctx context.Context, compHost component.Host) error {
	return nil
}

func (r *purefaLogsReceiver) Shutdown(ctx context.Context) error {

	return nil
}
