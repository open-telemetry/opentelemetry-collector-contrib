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

package aerospikereceiver_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver"
)

func TestNewFactory(t *testing.T) {
	factory := aerospikereceiver.NewFactory()
	require.Equal(t, "aerospike", string(factory.Type()))
	cfg := factory.CreateDefaultConfig().(*aerospikereceiver.Config)
	require.Equal(t, time.Minute, cfg.CollectionInterval)
	require.False(t, cfg.CollectClusterMetrics)
}
