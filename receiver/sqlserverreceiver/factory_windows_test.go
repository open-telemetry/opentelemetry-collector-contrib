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

//go:build windows
// +build windows

package sqlserverreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestCreateMetricsReceiver(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "creates a new factory and CreateMetricReceiver returns no error",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				r, err := factory.CreateMetricsReceiver(
					context.Background(),
					componenttest.NewNopReceiverCreateSettings(),
					cfg,
					consumertest.NewNop(),
				)
				require.NoError(t, err)
				require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()))
				require.NoError(t, r.Shutdown(context.Background()))
			},
		},
		{
			desc: "creates a new factory and CreateMetricReceiver returns error with incorrect config",
			testFunc: func(t *testing.T) {
				factory := NewFactory()
				_, err := factory.CreateMetricsReceiver(
					context.Background(),
					componenttest.NewNopReceiverCreateSettings(),
					nil,
					consumertest.NewNop(),
				)
				require.ErrorIs(t, err, errConfigNotSqlServer)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}
