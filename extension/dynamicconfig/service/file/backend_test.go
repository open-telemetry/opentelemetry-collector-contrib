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

package file

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestNewFileConfig(t *testing.T) {
	if _, err := NewBackend("woot.txt"); err == nil {
		t.Errorf("failed to catch nonexistant config file")
	}

	if config, err := NewBackend("../../testdata/schedules_bad.yaml"); err == nil {
		t.Errorf("failed to catch improper config file, built config: %v", config)
	}

	if _, err := NewBackend("../../testdata/schedules.yaml"); err != nil {
		t.Fatalf("failed to read config file")
	}
}

func TestUpdateConfig(t *testing.T) {
	originalSchedule := `ConfigBlocks:
    Schedules:
        - Period: MIN_5`
	updatedSchedule := `ConfigBlocks:
    Schedules:
        - Period: MIN_1`

	tmpfile := newTmpSchedule(t)
	defer os.Remove(tmpfile.Name())

	writeString(t, tmpfile, originalSchedule)

	backend, err := NewBackend(tmpfile.Name())
	if err != nil {
		t.Errorf("fail to create backend: %v", err)
	}
	backend.updateCh = make(chan struct{})

	if backend.configModel.ConfigBlocks[0].Schedules[0].Period != "MIN_5" {
		t.Errorf("update incorrect: wanted Period=MIN_5, got Schedules: %v",
			backend.configModel.ConfigBlocks[0])
	}

	writeString(t, tmpfile, updatedSchedule)
	timeout := makeTimeout(5 * time.Second)

	select {
	case <-backend.updateCh:
		if backend.configModel.ConfigBlocks[0].Schedules[0].Period != "MIN_1" {
			t.Errorf("update incorrect: wanted Period=MIN_1, got Schedules: %v",
				backend.configModel.ConfigBlocks[0])
		}
	case <-timeout:
		t.Errorf("local config update timed out")
	}
}

func newTmpSchedule(t *testing.T) *os.File {
	tmpfile, err := ioutil.TempFile("", "schedule.*.yaml")
	if err != nil {
		t.Fatalf("cannot open tempfile: %v", err)
	}

	return tmpfile
}

func writeString(t *testing.T, tmpfile *os.File, text string) {
	if _, err := tmpfile.Seek(0, 0); err != nil {
		t.Fatalf("cannot seek to beginning: %v", err)
	}

	if err := tmpfile.Truncate(0); err != nil {
		t.Fatalf("cannot truncate: %v", err)
	}

	if _, err := tmpfile.WriteString(text); err != nil {
		tmpfile.Close()
		t.Errorf("cannot write schedule: %v", err)
	}
}

func makeTimeout(dur time.Duration) <-chan struct{} {
	timeout := make(chan struct{}, 1)
	go func() {
		time.Sleep(dur)
		timeout <- struct{}{}
	}()

	return timeout
}

func TestFingerprint(t *testing.T) {
	backend, err := NewBackend("../../testdata/schedules.yaml")
	if err != nil {
		t.Fatalf("failed to read config file")
	}

	fingerprint := backend.configModel.ConfigBlocks[0].Hash()
	resp, err := backend.BuildConfigResponse(nil)
	if err != nil {
		t.Errorf("fail to build config response: %v", err)
	}

	if !bytes.Equal(fingerprint, resp.Fingerprint) {
		t.Errorf("fingerprint inconsistent: expected %v, got %v",
			fingerprint, resp.Fingerprint)
	}
}

func TestBuildConfigResponse(t *testing.T) {
	backend, err := NewBackend("../../testdata/schedules.yaml")
	if err != nil {
		t.Fatalf("failed to read config file")
	}

	resp, err := backend.BuildConfigResponse(nil)
	if err != nil {
		t.Errorf("fail to build config response: %v", err)
	}

	if resp.Fingerprint == nil || resp.Schedules == nil || resp.SuggestedWaitTimeSec == 0 {
		t.Errorf("config response incomplete: %v", resp)
	}
}

func TestBuildConfigResponseWithDuplicateInPattern(t *testing.T) {
	backend, err := NewBackend("../../testdata/schedules_improper_pattern.yaml")
	if err != nil {
		t.Fatalf("failed to read config file")
	}

	_, err = backend.BuildConfigResponse(nil)
	if err == nil {
		t.Errorf("fail to catch improper pattern")
	}
}

func TestWaitTime(t *testing.T) {
	backend := &Backend{}
	backend.SetWaitTime(3)

	if backend.GetWaitTime() != 3 {
		t.Errorf("fail to set waittime: expected %v, got %v", 3, backend.waitTime)
	}

	backend.SetWaitTime(-1)

	if backend.GetWaitTime() != 3 {
		t.Errorf("fail to catch incorrect waittime: expected %v, got %v", 3, backend.waitTime)
	}
}
