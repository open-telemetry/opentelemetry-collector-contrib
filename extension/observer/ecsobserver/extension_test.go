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

package ecsobserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

// Simply start and stop, the actual test logic is in sd_test.go until we implement the ListWatcher interface.
// In that case sd itself does not use timer and relies on caller to trigger List.
func TestExtensionStartStop(t *testing.T) {
	logger := zap.NewExample()
	sd, err := NewDiscovery(ExampleConfig(), ServiceDiscoveryOptions{
		Logger: logger,
		FetcherOverride: newMockFetcher(func() ([]*Task, error) {
			return nil, nil
		}),
	})
	require.NoError(t, err)
	ext := ecsObserver{logger: zap.NewExample(), sd: sd}
	require.NoError(t, ext.Start(context.TODO(), componenttest.NewNopHost()))
	require.NoError(t, ext.Shutdown(context.TODO()))
}
