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

package awsecscontainermetrics

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

func TestContainerResource(t *testing.T) {
	cm := ContainerMetadata{
		ContainerName: "container-1",
		DockerID:      "001",
		DockerName:    "docker-container-1",
	}

	r := containerResource(cm)
	require.NotNil(t, r)

	attrMap := r.Attributes()
	require.EqualValues(t, 3, attrMap.Len())

	expected := map[string]string{
		conventions.AttributeContainerName: "container-1",
		conventions.AttributeContainerID:   "001",
		AttributeECSDockerName:             "docker-container-1",
	}

	verifyAttributeMap(t, expected, attrMap)
}

func TestTaskResource(t *testing.T) {
	tm := TaskMetadata{
		Cluster:  "cluster-1",
		TaskARN:  "arn:aws:some-value/001",
		Family:   "task-def-family-1",
		Revision: "v1.2",
	}
	r := taskResource(tm)
	require.NotNil(t, r)

	attrMap := r.Attributes()
	require.EqualValues(t, 6, attrMap.Len())

	expected := map[string]string{
		AttributeECSCluster:      "cluster-1",
		AttributeECSTaskARN:      "arn:aws:some-value/001",
		AttributeECSTaskID:       "001",
		AttributeECSTaskFamily:   "task-def-family-1",
		AttributeECSTaskRevesion: "v1.2",
	}

	verifyAttributeMap(t, expected, attrMap)
}

func TestTaskResourceWithClusterARN(t *testing.T) {
	tm := TaskMetadata{
		Cluster:  "arn:aws:ecs:us-west-2:803860917211:cluster/main-cluster",
		TaskARN:  "arn:aws:some-value/001",
		Family:   "task-def-family-1",
		Revision: "v1.2",
	}
	r := taskResource(tm)
	require.NotNil(t, r)

	attrMap := r.Attributes()
	require.EqualValues(t, 6, attrMap.Len())

	expected := map[string]string{
		AttributeECSCluster:      "main-cluster",
		AttributeECSTaskARN:      "arn:aws:some-value/001",
		AttributeECSTaskID:       "001",
		AttributeECSTaskFamily:   "task-def-family-1",
		AttributeECSTaskRevesion: "v1.2",
	}

	verifyAttributeMap(t, expected, attrMap)
}

func verifyAttributeMap(t *testing.T, expected map[string]string, found pdata.AttributeMap) {
	for key, val := range expected {
		attributeVal, found := found.Get(key)
		require.EqualValues(t, true, found)

		require.EqualValues(t, val, attributeVal.StringVal())
	}
}
func TestGetTaskIDFromARN(t *testing.T) {
	id := getResourceFromARN("arn:aws:something/001")
	require.EqualValues(t, "001", id)

	id = getResourceFromARN("not-arn:aws:something/001")
	require.EqualValues(t, "not-arn:aws:something/001", id)

	id = getResourceFromARN("")
	require.LessOrEqual(t, 0, len(id))
}
