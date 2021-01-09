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

package ecsobserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestToPrometheusTarget(t *testing.T) {
	testCases := []struct {
		testName   string
		target     *Target
		promTarget *PrometheusTarget
	}{
		{
			"valid ip and port w/o labels",
			&Target{
				IP:   "10.0.0.129",
				Port: 9090,
			},
			&PrometheusTarget{
				Targets: []string{"10.0.0.129:9090"},
			},
		},
		{
			"valid ip and port w/ labels",
			&Target{
				IP:   "10.0.0.129",
				Port: 9090,
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
			},
			&PrometheusTarget{
				Targets: []string{"10.0.0.129:9090"},
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			promTarget := tc.target.toPrometheusTarget()
			assert.Equal(t, tc.promTarget, promTarget)
		})
	}
}

func TestToEndpoint(t *testing.T) {
	testCases := []struct {
		testName string
		target   *Target
		endpoint *observer.Endpoint
	}{
		{
			"valid ip and port w/o metrics path or labels",
			&Target{
				IP:   "10.0.0.129",
				Port: 9090,
			},
			&observer.Endpoint{
				ID:     observer.EndpointID("10.0.0.129:9090"),
				Target: "10.0.0.129",
				Details: observer.Task{
					Port: 9090,
				},
			},
		},
		{
			"valid ip, port and metrics path w/o labels",
			&Target{
				IP:          "10.0.0.129",
				Port:        9090,
				MetricsPath: "/metrics",
			},
			&observer.Endpoint{
				ID:     observer.EndpointID("10.0.0.129:9090/metrics"),
				Target: "10.0.0.129",
				Details: observer.Task{
					Port:        9090,
					MetricsPath: "/metrics",
				},
			},
		},
		{
			"valid ip, port, metrics path, and labels",
			&Target{
				IP:          "10.0.0.129",
				Port:        9090,
				MetricsPath: "/metrics",
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
			},
			&observer.Endpoint{
				ID:     observer.EndpointID("10.0.0.129:9090/metrics"),
				Target: "10.0.0.129",
				Details: observer.Task{
					Port:        9090,
					MetricsPath: "/metrics",
					Labels: map[string]string{
						"label1": "value1",
						"label2": "value2",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			endpoint := tc.target.toEndpoint()
			assert.Equal(t, tc.endpoint, endpoint)
		})
	}
}

func TestInitProcessors(t *testing.T) {
	sd := serviceDiscovery{
		config: &Config{
			ExtensionSettings: configmodels.ExtensionSettings{
				TypeVal: "ecs_observer",
				NameVal: "ecs_observer/1",
			},
			RefreshInterval: 15 * time.Second,
			ClusterName:     "EC2-Testing",
			ClusterRegion:   "us-west-2",
			ResultFile:      "/opt/aws/amazon-cloudwatch-agent/etc/ecs_sd_targets.yaml",
			DockerLabel: &DockerLabelConfig{
				JobNameLabel:     "ECS_PROMETHEUS_JOB_NAME",
				MetricsPathLabel: "ECS_PROMETHEUS_METRICS_PATH",
				PortLabel:        "ECS_PROMETHEUS_EXPORTER_PORT_SUBSET_A",
			},
			TaskDefinitions: []*TaskDefinitionConfig{
				{
					JobName:           "task_def_1",
					MetricsPath:       "/stats/metrics",
					MetricsPorts:      "9901;9404;9406",
					TaskDefArnPattern: ".*:task-definition/bugbash-java-fargate-awsvpc-task-def-only:[0-9]+",
				},
				{
					ContainerNamePattern: "^bugbash-jar.*$",
					MetricsPorts:         "9902",
					TaskDefArnPattern:    ".*:task-definition/nginx:[0-9]+",
				},
			},
			logger: zap.NewNop(),
		},
	}

	sd.initProcessors()

	expectedProcessorNames := []string{
		"TaskRetrievalProcessor",
		"TaskDefinitionProcessor",
		"TaskFilterProcessor",
		"MetadataProcessor",
	}

	for i, p := range sd.processors {
		assert.Equal(t, expectedProcessorNames[i], p.ProcessorName())
	}
}
