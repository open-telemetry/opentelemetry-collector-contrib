// Copyright The OpenTelemetry Authors
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

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr        = "cloudflare"
	stabilityLevel = component.StabilityLevelAlpha
)

// NewFactory returns the component factory for the cloudflarereceiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, stabilityLevel),
	)
}

func createLogsReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	rConf component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg := rConf.(*Config)
	return newLogsReceiver(params, cfg, consumer)
}

func createDefaultConfig() component.Config {
	return &Config{
		Logs: LogsConfig{
			TimestampField: defaultTimestampField,
			TLS:            &configtls.TLSServerSetting{},
		},
	}
}
