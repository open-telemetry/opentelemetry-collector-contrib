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

import "time"

// Provides a server start time for cumulative metrics and the current time
// for all metrics. The server start time is calculated by subtracting the
// uptime from the current time. This is done during construction or if a server
// restart is detected. Server restarts are detected by testing for a lower
// uptime value than the most recent uptime. Provides the current (server) time
// by adding the uptime value to the server start time.
type timeBundle struct {
	serverStart       time.Time
	current           time.Time
	lastUptimeSeconds int
}

func newTimeBundle(now time.Time, uptimeSeconds int) *timeBundle {
	serverStart := calculateServerStart(now, uptimeSeconds)
	return &timeBundle{
		serverStart:       serverStart,
		lastUptimeSeconds: uptimeSeconds,
		current:           now,
	}
}

// Calculates the current time by adding the server uptime to the server start
// time. If a server restart is detected, updates the server start time.
func (t *timeBundle) update(now time.Time, uptimeSeconds int) {
	if uptimeSeconds < t.lastUptimeSeconds {
		// assume a server restart
		t.lastUptimeSeconds = uptimeSeconds
		t.serverStart = calculateServerStart(now, uptimeSeconds)
	}
	t.current = t.serverStart.Add(time.Duration(uptimeSeconds) * time.Second)
}

func calculateServerStart(now time.Time, uptimeSeconds int) time.Time {
	return now.Add(time.Duration(-uptimeSeconds) * time.Second)
}
