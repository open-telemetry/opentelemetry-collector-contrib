// Copyright 2020, OpenTelemetry Authors
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

package redisreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStartTime(t *testing.T) {
	startTime := newTimeBundle(time.Unix(1000, 0), 60)
	require.Equal(t, time.Unix(940, 0), startTime.serverStart)

	startTime.update(time.Unix(1010, 0), 70)
	require.Equal(t, time.Unix(1010, 0), startTime.current)

	// Simulate a server restart, which would cause a lower uptime
	startTime.update(time.Unix(1030, 0), 10)
	require.Equal(t, time.Unix(1020, 0), startTime.serverStart)
	require.Equal(t, time.Unix(1030, 0), startTime.current)

	startTime.update(time.Unix(1040, 0), 20)
	require.Equal(t, time.Unix(1020, 0), startTime.serverStart)
	require.Equal(t, time.Unix(1040, 0), startTime.current)

	startTime.update(time.Unix(1050, 0), 25)
	require.Equal(t, time.Unix(1045, 0), startTime.current)
}
