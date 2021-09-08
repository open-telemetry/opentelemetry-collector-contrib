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

package kubeletstatsreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

func TestMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	o, err := cfg.getReceiverOptions()
	require.NoError(t, err)

	metricsReceiver := newReceiver(
		o, zap.NewNop(), &fakeRestClient{},
		consumertest.NewNop(),
	)
	ctx := context.Background()
	err = metricsReceiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)
	err = metricsReceiver.Shutdown(ctx)
	require.NoError(t, err)
}
