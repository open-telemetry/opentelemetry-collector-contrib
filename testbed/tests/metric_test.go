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

package tests

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/testbed/testbed"
	scenarios "github.com/open-telemetry/opentelemetry-collector/testbed/tests"
)

func TestMetric10kDPS(t *testing.T) {
	tests := []struct {
		name         string
		sender       testbed.DataSender
		receiver     testbed.DataReceiver
		resourceSpec testbed.ResourceSpec
	}{
		{
			"OpenCensus",
			testbed.NewOCMetricDataSender(testbed.GetAvailablePort(t)),
			testbed.NewOCDataReceiver(testbed.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 40,
				ExpectedMaxRAM: 70,
			},
		},
		{
			"Carbon",
			NewCarbonDataSender(testbed.GetAvailablePort(t)),
			NewCarbonDataReceiver(testbed.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 200,
				ExpectedMaxRAM: 65,
			},
		},
		{
			"SignalFx",
			NewSFxMetricDataSender(testbed.GetAvailablePort(t)),
			NewSFxMetricsDataReceiver(testbed.GetAvailablePort(t)),
			testbed.ResourceSpec{
				ExpectedMaxCPU: 80,
				ExpectedMaxRAM: 80,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scenarios.Scenario10kItemsPerSecond(
				t,
				test.sender,
				test.receiver,
				test.resourceSpec,
				nil,
			)
		})
	}

}
