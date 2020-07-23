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
	"os/exec"
	"testing"
)

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Log("warning: not recompiling binaries: omit -short flag to compile new binaries")
	} else {
		t.Log("building new collector")
		buildCollector(t)

		t.Log("building new sample app")
		buildSampleApp(t)
	}

	testFileBackend(t)
	testRemoteBackend(t)
}

func buildCollector(t *testing.T) {
	cmd := exec.Command("make", "otelcontribcol")
	cmd.Dir = "../../../" // run in top-level of repo

	if err := cmd.Run(); err != nil {
		t.Fatalf("fail to compile otelcontribcol: %v", err)
	}
}

func buildSampleApp(t *testing.T) {
	cmd := exec.Command("go", "build", "main.go")
	cmd.Dir = "app"

	if err := cmd.Run(); err != nil {
		t.Fatalf("fail to compile sample app: %v", err)
	}
}
