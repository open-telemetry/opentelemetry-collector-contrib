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
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewDiscovery(t *testing.T) {

}

// Util Start

func newTestFilter(t *testing.T, cfg Config) *taskFilter {
	logger := zap.NewExample()
	m, err := newMatchers(cfg, MatcherOptions{Logger: logger})
	require.NoError(t, err)
	f := newTaskFilter(logger, m)
	return f
}

func newMatcher(t *testing.T, cfg matcherConfig) Matcher {
	m, err := cfg.newMatcher(testMatcherOptions())
	require.NoError(t, err)
	return m
}

func newMatcherAndMatch(t *testing.T, cfg matcherConfig, tasks []*Task) *MatchResult {
	m := newMatcher(t, cfg)
	res, err := matchContainers(tasks, m, 0)
	require.NoError(t, err)
	return res
}

func testMatcherOptions() MatcherOptions {
	return MatcherOptions{
		Logger: zap.NewExample(),
	}
}

// Util End
