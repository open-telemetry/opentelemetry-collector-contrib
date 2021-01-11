// Copyright The OpenTelemetry Authors
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

package spanmetricsprocessor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/mocks"
)

func TestProcessorStart(t *testing.T) {
	p := &processorImp{
		logger: zap.NewNop(),
	}
	assert.NoError(t, p.Start(context.Background(), nil))

	// Verify
	// TODO: Add verification once processor logic is in place
}

func TestProcessorShutdown(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Test
	next := new(consumertest.TracesSink)
	p := newProcessor(zap.NewNop(), cfg, next)
	require.NotNil(t, p)
	// Verify
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestProcessorCapabilities(t *testing.T) {
	// Prepare
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	// Test
	next := new(consumertest.TracesSink)
	p := newProcessor(zap.NewNop(), cfg, next)
	require.NotNil(t, p)
	caps := p.GetCapabilities()

	// Verify
	assert.False(t, caps.MutatesConsumedData)
	assert.NoError(t, p.Shutdown(context.Background()))
}

func TestProcessorConsumeTraces(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		wantConsumeMetricsErr string
		wantConsumeTracesErr  string
	}{
		{name: "success"},
		{name: "consume metrics error", wantConsumeMetricsErr: "consume metrics error"},
		{name: "consume traces error", wantConsumeTracesErr: "consume traces error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			mexp := &mocks.MetricsExporter{}
			tcon := &mocks.TracesConsumer{}

			// TODO: Add verification of metrics input to ConsumeMetrics
			consumeMetricsErr := error(nil)
			if tc.wantConsumeMetricsErr != "" {
				consumeMetricsErr = fmt.Errorf(tc.wantConsumeMetricsErr)
			}
			mexp.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(consumeMetricsErr)

			consumeTracesErr := error(nil)
			if tc.wantConsumeTracesErr != "" {
				consumeTracesErr = fmt.Errorf(tc.wantConsumeTracesErr)
			}
			tcon.On("ConsumeTraces", mock.Anything, mock.Anything).Return(consumeTracesErr)

			p := &processorImp{
				logger:          zap.NewNop(),
				metricsExporter: mexp,
				nextConsumer:    tcon,
			}
			traces := pdata.NewTraces()

			// Test
			ctx := metadata.NewIncomingContext(context.Background(), nil)
			err := p.ConsumeTraces(ctx, traces)

			// Verify
			if tc.wantConsumeMetricsErr != "" {
				assert.EqualError(t, err, tc.wantConsumeMetricsErr)
			} else if tc.wantConsumeTracesErr != "" {
				assert.EqualError(t, err, tc.wantConsumeTracesErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
