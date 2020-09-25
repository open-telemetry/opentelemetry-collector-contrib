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

	labels := r.Labels
	require.EqualValues(t, 3, len(labels))

	require.EqualValues(t, "container-1", labels[conventions.AttributeContainerName])
	require.EqualValues(t, "001", labels[conventions.AttributeContainerID])
	require.EqualValues(t, "docker-container-1", labels[AttributeECSDockerName])
}

func TestTaskResource(t *testing.T) {
	tm := TaskMetadata{
		Cluster:  "cluster-1",
		TaskARN:  "arn:aws:some-value/001",
		Family:   "task-def-family-1",
		Revision: "task-def-version-1",
	}
	r := taskResource(tm)
	require.NotNil(t, r)

	labels := r.Labels
	require.EqualValues(t, 6, len(labels))

	require.EqualValues(t, "cluster-1", labels[AttributeECSCluster])
	require.EqualValues(t, "arn:aws:some-value/001", labels[AttributeECSTaskARN])
	require.EqualValues(t, "001", labels[AttributeECSTaskID])
	require.EqualValues(t, "task-def-family-1", labels[AttributeECSTaskFamily])
	require.EqualValues(t, "task-def-version-1", labels[AttributeECSTaskRevesion])
}

func TestGetTaskIDFromARN(t *testing.T) {
	id := getTaskIDFromARN("arn:aws:something/001")
	require.EqualValues(t, "001", id)

	id = getTaskIDFromARN("not-arn:aws:something/001")
	require.LessOrEqual(t, 0, len(id))

	id = getTaskIDFromARN("")
	require.LessOrEqual(t, 0, len(id))
}
