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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

func TestExtension(t *testing.T) {
	logger := zap.NewNop()
	config := &config{}

	extension := newJmxMetricsExtension(logger, config)
	assert.NotNil(t, extension)
	assert.Same(t, logger, extension.logger)
	assert.Same(t, config, extension.config)

	assert.Nil(t, extension.Start(context.Background(), componenttest.NewNopHost()))
	assert.Nil(t, extension.Shutdown(context.Background()))
	assert.Nil(t, extension.Ready())
	assert.Nil(t, extension.NotReady())
}
