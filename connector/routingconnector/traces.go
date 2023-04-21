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

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type tracesConnector struct {
	logger *zap.Logger
	config *Config
	component.StartFunc
	component.ShutdownFunc
}

func newTracesConnector(set connector.CreateSettings, cfg component.Config, traces consumer.Traces) (*tracesConnector, error) {
	return &tracesConnector{
		logger: set.TelemetrySettings.Logger,
		config: cfg.(*Config),
	}, nil
}

func (tc *tracesConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (tc *tracesConnector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return nil
}
