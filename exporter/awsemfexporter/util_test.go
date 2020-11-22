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

package awsemfexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestReplacePatternValidTaskId(t *testing.T) {
	logger := zap.NewNop()

	input := "{TaskId}"

	attrMap := pdata.NewAttributeMap()
	attrMap.InitEmptyWithCapacity(2)

	attrMap.UpsertString("aws.ecs.cluster.name", "test-cluster-name")
	attrMap.UpsertString("aws.ecs.task.id", "test-task-id")

	s := replacePatterns(input, attrMap, logger)

	assert.Equal(t, "test-task-id", s)
}

func TestReplacePatternValidClusterName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := pdata.NewAttributeMap()
	attrMap.InitEmptyWithCapacity(2)

	attrMap.UpsertString("aws.ecs.cluster.name", "test-cluster-name")
	attrMap.UpsertString("aws.ecs.task.id", "test-task-id")

	s := replacePatterns(input, attrMap, logger)

	assert.Equal(t, "/aws/ecs/containerinsights/test-cluster-name/performance", s)
}

func TestReplacePatternMissingAttribute(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := pdata.NewAttributeMap()
	attrMap.InitEmptyWithCapacity(1)

	attrMap.UpsertString("aws.ecs.task.id", "test-task-id")

	s := replacePatterns(input, attrMap, logger)

	assert.Equal(t, "/aws/ecs/containerinsights/undefined/performance", s)
}

func TestReplacePatternAttrPlaceholderClusterName(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := pdata.NewAttributeMap()
	attrMap.InitEmptyWithCapacity(1)

	attrMap.UpsertString("ClusterName", "test-cluster-name")

	s := replacePatterns(input, attrMap, logger)

	assert.Equal(t, "/aws/ecs/containerinsights/test-cluster-name/performance", s)
}

func TestReplacePatternWrongKey(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{WrongKey}/performance"

	attrMap := pdata.NewAttributeMap()
	attrMap.InitEmptyWithCapacity(1)

	attrMap.UpsertString("ClusterName", "test-task-id")

	s := replacePatterns(input, attrMap, logger)

	assert.Equal(t, "/aws/ecs/containerinsights/{WrongKey}/performance", s)
}

func TestReplacePatternNilAttrValue(t *testing.T) {
	logger := zap.NewNop()

	input := "/aws/ecs/containerinsights/{ClusterName}/performance"

	attrMap := pdata.NewAttributeMap()
	attrMap.InsertNull("ClusterName")

	s := replacePatterns(input, attrMap, logger)

	assert.Equal(t, "/aws/ecs/containerinsights/undefined/performance", s)
}
