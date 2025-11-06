// Copyright observIQ, Inc.
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

package macosunifiedloggingreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateDefaultConfig(t *testing.T) {
	expectedCfg := &Config{
		MaxPollInterval: 30 * time.Second,
		MaxLogAge:       24 * time.Hour,
		Format:          "default",
	}

	componentCfg := createDefaultConfig()
	actualCfg, ok := componentCfg.(*Config)
	require.True(t, ok)
	require.Equal(t, expectedCfg, actualCfg)
}
