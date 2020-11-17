// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

const (
	AttributeCluster   = "ecs.cluster"
	AttributeECSTaskID = "ecs.task-id"
)

func replacePatterns(s string, attrMap pdata.AttributeMap, logger *zap.Logger) string {
	s = replacePatternWithResource(s, AttributeCluster, attrMap, logger)
	s = replacePatternWithResource(s, AttributeECSTaskID, attrMap, logger)
	return s
}

func replacePatternWithResource(s, patternKey string, attrMap pdata.AttributeMap, logger *zap.Logger) string {
	pattern := "{{" + patternKey + "}}"
	if strings.Contains(s, pattern) {
		value, ok := attrMap.Get(patternKey)
		if ok {
			if !value.IsNil() {
				stringVal := value.StringVal()
				s = strings.ReplaceAll(s, pattern, stringVal)
			}
		} else {
			logger.Error("No resource attribute found for pattern " + pattern)
		}
	}
	return s
}
