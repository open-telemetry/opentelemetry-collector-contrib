// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jmxmetricsextension

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

var _ component.ServiceExtension = (*jmxMetricsExtension)(nil)
var _ component.PipelineWatcher = (*jmxMetricsExtension)(nil)

type jmxMetricsExtension struct {
	logger *zap.Logger
	config *config
}

func newJmxMetricsExtension(
	logger *zap.Logger,
	config *config,
) *jmxMetricsExtension {
	return &jmxMetricsExtension{
		logger: logger,
		config: config,
	}
}

func (jmx *jmxMetricsExtension) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (jmx *jmxMetricsExtension) Shutdown(ctx context.Context) error {
	return nil
}

func (jmx *jmxMetricsExtension) Ready() error {
	return nil
}

func (jmx *jmxMetricsExtension) NotReady() error {
	return nil
}
