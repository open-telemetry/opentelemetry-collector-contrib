// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package attributes

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

func TestTagsFromAttributes(t *testing.T) {
	attributeMap := map[string]pdata.AttributeValue{
		conventions.AttributeProcessExecutableName: pdata.NewAttributeValueString("otelcol"),
		conventions.AttributeProcessExecutablePath: pdata.NewAttributeValueString("/usr/bin/cmd/otelcol"),
		conventions.AttributeProcessCommand:        pdata.NewAttributeValueString("cmd/otelcol"),
		conventions.AttributeProcessCommandLine:    pdata.NewAttributeValueString("cmd/otelcol --config=\"/path/to/config.yaml\""),
		conventions.AttributeProcessPID:            pdata.NewAttributeValueInt(1),
		conventions.AttributeProcessOwner:          pdata.NewAttributeValueString("root"),
		conventions.AttributeOSType:                pdata.NewAttributeValueString("LINUX"),
		conventions.AttributeK8SDaemonSetName:      pdata.NewAttributeValueString("daemon_set_name"),
		conventions.AttributeAWSECSClusterARN:      pdata.NewAttributeValueString("cluster_arn"),
		"tags.datadoghq.com/service":               pdata.NewAttributeValueString("service_name"),
	}
	attrs := pdata.NewAttributeMapFromMap(attributeMap)

	assert.ElementsMatch(t, []string{
		fmt.Sprintf("%s:%s", conventions.AttributeProcessExecutableName, "otelcol"),
		fmt.Sprintf("%s:%s", conventions.AttributeOSType, "LINUX"),
		fmt.Sprintf("%s:%s", "kube_daemon_set", "daemon_set_name"),
		fmt.Sprintf("%s:%s", "ecs_cluster_name", "cluster_arn"),
		fmt.Sprintf("%s:%s", "service", "service_name"),
	}, TagsFromAttributes(attrs))
}

func TestTagsFromAttributesEmpty(t *testing.T) {
	attrs := pdata.NewAttributeMap()

	assert.Equal(t, []string{}, TagsFromAttributes(attrs))
}
