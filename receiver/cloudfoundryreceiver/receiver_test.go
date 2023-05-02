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

package cloudfoundryreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// Test to make sure a new receiver can be created properly, started and shutdown with the default config
func TestDefaultValidReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	params := receivertest.NewNopCreateSettings()

	receiver, err := newCloudFoundryReceiver(
		params,
		*cfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, receiver, "receiver creation failed")

	// Test start
	ctx := context.Background()
	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	// Test shutdown
	err = receiver.Shutdown(ctx)
	require.NoError(t, err)
}

// Test to make sure start fails with invalid consumer
func TestInvalidConsumer(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	params := receivertest.NewNopCreateSettings()

	receiver, err := newCloudFoundryReceiver(
		params,
		*cfg,
		nil,
	)

	require.EqualError(t, err, "nil next Consumer")
	require.Nil(t, receiver, "receiver creation failed")
}
