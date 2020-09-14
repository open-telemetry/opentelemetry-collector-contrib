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

package subprocess

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSubprocessAndConfig(t *testing.T) {
	logger := zap.NewNop()
	config := &Config{}
	subprocess := NewSubprocess(config, logger)
	require.NotNil(t, subprocess)
	require.Same(t, config, subprocess.config)
	require.Same(t, logger, subprocess.logger)
	require.NotNil(t, subprocess.Stdout)
}

func TestShutdownTimeout(t *testing.T) {
	logger := zap.NewNop()
	subprocess := NewSubprocess(nil, logger)
	require.NotNil(t, subprocess)

	_, cancel := context.WithCancel(context.Background())
	subprocess.cancel = cancel

	t0 := time.Now()
	err := subprocess.Shutdown(context.Background())
	require.NoError(t, err)

	elapsed := int64(time.Since(t0))
	require.GreaterOrEqual(t, elapsed, int64(5*time.Second))
}
