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

// +build integration

package test

import (
	"testing"
	"time"
)

func testFileBackend(t *testing.T) {
	t.Log("starting file backend test")

	schedFile := getSchedulesFile(t)
	writeString(t, schedFile, sec1Schedule)

	t.Log("starting file backend collector")
	backendCmd, stderr := startCollector(t, "testdata/file-backend-config.yaml", ":8888")
	defer backendCmd.Process.Kill()

	t.Log("starting sample application")
	appCmd := startSampleApp(t)
	defer appCmd.Process.Kill()

	t.Log("capturing logs for period=SEC_1")
	avgDuration := timeLogs(t, stderr, 10, 10)
	t.Log("avg duration:", avgDuration)
	if !fuzzyEqualDuration(avgDuration, time.Second, 999*time.Millisecond) {
		t.Errorf("expected period=SEC_1, got: %v", avgDuration)
	}

	writeString(t, schedFile, sec5Schedule)

	t.Log("propogating period=SEC_5")
	time.Sleep(6 * time.Second)

	t.Log("capturing logs for period=SEC_5")
	avgDuration = timeLogs(t, stderr, 10, 10)
	t.Log("avg duration:", avgDuration)
	if !fuzzyEqualDuration(avgDuration, 5*time.Second, 999*time.Millisecond) {
		t.Errorf("expected period=SEC_5, got: %v", avgDuration)
	}
}
