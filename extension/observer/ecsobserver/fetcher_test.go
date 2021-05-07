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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver/internal/ecsmock"
)

func TestFetcher_GetAllTasks(t *testing.T) {
	c := ecsmock.NewCluster()
	f, err := newTaskFetcher(taskFetcherOptions{
		Logger:      zap.NewExample(),
		Cluster:     "not used",
		Region:      "not used",
		ecsOverride: c,
	})
	require.NoError(t, err)
	c.SetTasks(ecsmock.GenTasks("p", 203))
	ctx := context.Background()
	tasks, err := f.GetAllTasks(ctx)
	require.NoError(t, err)
	assert.Equal(t, 203, len(tasks))
}
