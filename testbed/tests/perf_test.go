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
// TESTBED_CONFIG=local.yaml go test -v

package tests

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/testbed/testbed"
	scenarios "github.com/open-telemetry/opentelemetry-collector/testbed/tests"
	"github.com/open-telemetry/opentelemetry-collector/testutils"
)

// TestMain is used to initiate setup, execution and tear down of testbed.
func TestMain(m *testing.M) {
	testbed.DoTestMain(m)
}

func Test10kSPS(t *testing.T) {
	tests := []struct {
		name     string
		receiver testbed.Receiver
	}{
		{"JaegerRx", testbed.NewJaegerReceiver(int(testutils.GetAvailablePort(t)))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scenarios.Scenario10kSPS(t, testbed.NewJaegerExporter(int(testutils.GetAvailablePort(t))), test.receiver)
		})
	}
}
