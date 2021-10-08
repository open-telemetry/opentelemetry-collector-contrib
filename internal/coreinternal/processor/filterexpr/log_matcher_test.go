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

package filterexpr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestLogCompileExprError(t *testing.T) {
	_, err := NewMetricMatcher("")
	require.Error(t, err)
}

func TestLogRunExprError(t *testing.T) {
	matcher, err := NewMetricMatcher("foo")
	require.NoError(t, err)
	matched, _ := matcher.match(metricEnv{})
	require.False(t, matched)
}

func TestLogStringBodyMatches(t *testing.T) {
	matcher, err := NewLogMatcher(`Body matches 'my.log'`)
	require.NoError(t, err)
	l := pdata.NewLogRecord()
	l.Body().SetStringVal("my.log")
	matched, err := matcher.MatchLog(l)
	assert.NoError(t, err)
	assert.True(t, matched)
}

func TestLogStringodyDoesntMatch(t *testing.T) {
	matcher, err := NewLogMatcher(`Body matches 'my.logs'`)
	require.NoError(t, err)
	l := pdata.NewLogRecord()
	l.Body().SetStringVal("my.log")
	matched, err := matcher.MatchLog(l)
	assert.NoError(t, err)
	assert.False(t, matched)
}
