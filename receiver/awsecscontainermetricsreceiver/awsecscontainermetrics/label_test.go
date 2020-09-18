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
)

func TestContainerLabelKeysAndValues(t *testing.T) {
	cm := ContainerMetadata{
		ContainerName: "container-1",
		DockerID:      "001",
		DockerName:    "docker-container-1",
	}
	k, v := containerLabelKeysAndValues(cm)
	require.EqualValues(t, ContainerMetricsLabelLen, len(k))
	require.EqualValues(t, ContainerMetricsLabelLen, len(v))

	require.EqualValues(t, "container-1", v[0].Value)
	require.EqualValues(t, "001", v[1].Value)
	require.EqualValues(t, "docker-container-1", v[2].Value)
}

func TestTaskLabelKeysAndValues(t *testing.T) {
	tm := TaskMetadata{
		Cluster:  "cluster-1",
		TaskARN:  "arn:aws:some-value/001",
		Family:   "task-def-family-1",
		Revision: "task-def-version-1",
	}
	k, v := taskLabelKeysAndValues(tm)
	require.EqualValues(t, TaskMetricsLabelLen, len(k))
	require.EqualValues(t, TaskMetricsLabelLen, len(v))

	require.EqualValues(t, "cluster-1", v[0].Value)
	require.EqualValues(t, "arn:aws:some-value/001", v[1].Value)
	require.EqualValues(t, "001", v[2].Value)
	require.EqualValues(t, "task-def-family-1", v[3].Value)
	require.EqualValues(t, "task-def-version-1", v[4].Value)
}

func TestGetTaskIDFromARN(t *testing.T) {
	id := getTaskIDFromARN("arn:aws:something/001")
	require.EqualValues(t, "001", id)

	id = getTaskIDFromARN("not-arn:aws:something/001")
	require.LessOrEqual(t, 0, len(id))

	id = getTaskIDFromARN("")
	require.LessOrEqual(t, 0, len(id))
}
