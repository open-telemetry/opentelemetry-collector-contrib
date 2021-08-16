// Copyright The OpenTelemetry Authors
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

package awsxrayproxy

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

type xrayProxy struct {
	logger *zap.Logger
	config *Config
}

var _ component.Extension = (*xrayProxy)(nil)

func (x xrayProxy) Start(ctx context.Context, host component.Host) error {
	// TODO(anuraaga): Implement me
	return nil
}

func (x xrayProxy) Shutdown(ctx context.Context) error {
	// TODO(anuraaga): Implement me
	return nil
}

func newXrayProxy(config *Config, logger *zap.Logger) (component.Extension, error) {
	p := &xrayProxy{
		config: config,
		logger: logger,
	}

	return p, nil
}
