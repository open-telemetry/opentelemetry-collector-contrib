// Copyright 2019, OpenTelemetry Authors
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

// Package tests contains test cases. To run the tests go to tests directory and run:
// RUN_TESTBED=1 go test -v

package tests

import (
	"testing"

	"go.opentelemetry.io/collector/testbed/testbed"
)

var contribPerfResultsSummary testbed.TestResultsSummary = &testbed.PerformanceResults{}

// TestMain is used to initiate setup, execution and tear down of testbed.
func TestMain(m *testing.M) {
	testbed.GlobalConfig.DefaultAgentExeRelativeFile = "../../bin/otelcontribcol_{{.GOOS}}_{{.GOARCH}}"
	testbed.DoTestMain(m, contribPerfResultsSummary)
}

// This file is left with no tests as a placeholder for future unstable exe tests
