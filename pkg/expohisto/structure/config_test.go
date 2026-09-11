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

package structure // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/expohisto/structure"

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValid(t *testing.T) {
	require.True(t, Config{}.Valid())
	require.True(t, NewConfig().Valid())
	require.True(t, NewConfig(WithMaxSize(MinSize)).Valid())
	require.True(t, NewConfig(WithMaxSize(MaximumMaxSize)).Valid())
	require.True(t, NewConfig(WithMaxSize((MinSize+MaximumMaxSize)/2)).Valid())

	require.False(t, NewConfig(WithMaxSize(-1)).Valid())
	require.False(t, NewConfig(WithMaxSize(1<<20)).Valid())
	require.False(t, NewConfig(WithMaxSize(1)).Valid())

	require.True(t, NewConfig(WithMaxScale(0)).Valid())
	require.True(t, NewConfig(WithMaxScale(MinimumMaxScale)).Valid())
	require.True(t, NewConfig(WithMaxScale(MaximumMaxScale)).Valid())

	require.False(t, NewConfig(WithMaxScale(MinimumMaxScale-1)).Valid())
	require.False(t, NewConfig(WithMaxScale(MaximumMaxScale+1)).Valid())
}

func TestConfigMaxScaleDefault(t *testing.T) {
	cfg, err := Config{}.Validate()
	require.NoError(t, err)
	require.Equal(t, DefaultMaxScale, cfg.maxScale)

	// An explicit zero is honored, it is not treated as unset.
	cfg, err = NewConfig(WithMaxScale(0)).Validate()
	require.NoError(t, err)
	require.Equal(t, int32(0), cfg.maxScale)
}

func TestConfigMaxScaleClamped(t *testing.T) {
	cfg, err := NewConfig(WithMaxScale(MaximumMaxScale + 5)).Validate()
	require.Error(t, err)
	require.Equal(t, MaximumMaxScale, cfg.maxScale)

	cfg, err = NewConfig(WithMaxScale(MinimumMaxScale - 5)).Validate()
	require.Error(t, err)
	require.Equal(t, MinimumMaxScale, cfg.maxScale)
}

func TestConfigInvalidSizeAndScale(t *testing.T) {
	cfg, err := NewConfig(WithMaxSize(1), WithMaxScale(MaximumMaxScale+1)).Validate()
	require.Error(t, err)
	require.Equal(t, int32(MinSize), cfg.maxSize)
	require.Equal(t, MaximumMaxScale, cfg.maxScale)
}
