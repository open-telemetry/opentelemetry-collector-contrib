// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecstaskobserver

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/ecsutiltest"
)

func TestEndpointsFromTaskMetadata(t *testing.T) {
	config := defaultConfig()
	e := ecsTaskObserver{config: &config, telemetry: componenttest.NewNopTelemetrySettings()}

	var taskMetadata ecsutil.TaskMetadata
	require.NoError(t, json.Unmarshal(ecsutiltest.TaskMetadataTestResponse, &taskMetadata))

	expectedEndpoints := []observer.Endpoint{
		{
			ID:     observer.EndpointID("nginx100-5302b3fac16c62951717f444030cb1b8f233f40c03fe5507fc127ca1a70597da"),
			Target: "172.17.0.3",
			Details: &observer.Container{
				ContainerID: "5302b3fac16c62951717f444030cb1b8f233f40c03fe5507fc127ca1a70597da",
				Host:        "172.17.0.3",
				Image:       "nginx",
				Tag:         "latest",
				Labels: map[string]string{
					"com.amazonaws.ecs.cluster":                 "test200",
					"com.amazonaws.ecs.container-name":          "nginx100",
					"com.amazonaws.ecs.task-arn":                "arn:aws:ecs:us-west-2:803860917211:task/test200/d22aaa11bf0e4ab19c2c940a1cbabbee",
					"com.amazonaws.ecs.task-definition-family":  "three-nginx",
					"com.amazonaws.ecs.task-definition-version": "1",
				},
				Name:          "nginx100",
				Port:          0,
				AlternatePort: 0,
			},
		},
		{
			ID:     observer.EndpointID("nginx300-4a984770705c4f4f95e1267af3623ab0923c602b7cd4ed7d77b7f8356537337f"),
			Target: "172.17.0.4",
			Details: &observer.Container{
				ContainerID: "4a984770705c4f4f95e1267af3623ab0923c602b7cd4ed7d77b7f8356537337f",
				Host:        "172.17.0.4",
				Image:       "nginx",
				Tag:         "latest",
				Labels: map[string]string{
					"com.amazonaws.ecs.cluster":                 "test200",
					"com.amazonaws.ecs.container-name":          "nginx300",
					"com.amazonaws.ecs.task-arn":                "arn:aws:ecs:us-west-2:803860917211:task/test200/d22aaa11bf0e4ab19c2c940a1cbabbee",
					"com.amazonaws.ecs.task-definition-family":  "three-nginx",
					"com.amazonaws.ecs.task-definition-version": "1",
				},
				Name:          "nginx300",
				Port:          0,
				AlternatePort: 0,
			},
		},
	}

	endpoints := e.endpointsFromTaskMetadata(&taskMetadata)
	require.Equal(t, expectedEndpoints, endpoints)
}

func TestPortFromLabels(t *testing.T) {
	config := defaultConfig()
	e := ecsTaskObserver{config: &config, telemetry: componenttest.NewNopTelemetrySettings()}

	require.Equal(t, uint16(0), e.portFromLabels(nil))

	defaultValidLabel := map[string]string{
		"ECS_TASK_OBSERVER_PORT": "123",
	}
	require.Equal(t, uint16(123), e.portFromLabels(defaultValidLabel))

	e.config.PortLabels = []string{
		"MISSING_LABEL", "NONNUMERIC", "FLOAT", "INT64", "NEGATIVE", "VALID",
	}

	labels := map[string]string{
		"NONNUMERIC": "string", "FLOAT": "123.456",
		"INT64": "4294967297", "NEGATIVE": "-234",
		"VALID": "234",
	}
	require.Equal(t, uint16(234), e.portFromLabels(labels))
}
