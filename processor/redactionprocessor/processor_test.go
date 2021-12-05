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

package redactionprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap/zaptest"
)

func TestCapabilities(t *testing.T) {
	config := &Config{}
	next := new(consumertest.TracesSink)
	processor, err := newRedaction(context.Background(), config, zaptest.NewLogger(t), next)
	assert.NoError(t, err)

	cap := processor.Capabilities()
	assert.True(t, cap.MutatesData)
}

func TestStartShutdown(t *testing.T) {
	config := &Config{}
	next := new(consumertest.TracesSink)
	processor, err := newRedaction(context.Background(), config, zaptest.NewLogger(t), next)
	assert.NoError(t, err)

	ctx := context.Background()
	err = processor.Start(ctx, componenttest.NewNopHost())
	assert.Nil(t, err)
	err = processor.Shutdown(ctx)
	assert.Nil(t, err)
}
