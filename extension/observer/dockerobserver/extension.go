// Copyright  OpenTelemetry Authors
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

package dockerobserver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

var _ (component.Extension) = (*dockerObserver)(nil)

type dockerObserver struct {
	logger *zap.Logger
	config *Config
}

func (d *dockerObserver) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (d *dockerObserver) Shutdown(ctx context.Context) error {
	return nil
}

// newObserver creates a new docker observer extension.
func newObserver(logger *zap.Logger, config *Config) (component.Extension, error) {
	return &dockerObserver{logger: logger, config: config}, nil
}
