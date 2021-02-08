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
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
)

var patternKeyToAttributeMap = map[string]string{
	"ClusterName": "aws.ecs.cluster.name",
	"TaskId":      "aws.ecs.task.id",
}

func replacePatterns(s string, attrMap pdata.AttributeMap, logger *zap.Logger) string {
	for key := range patternKeyToAttributeMap {
		s = replacePatternWithResource(s, key, attrMap, logger)
	}
	return s
}

func replacePatternWithResource(s, patternKey string, attrMap pdata.AttributeMap, logger *zap.Logger) string {
	pattern := "{" + patternKey + "}"
	if strings.Contains(s, pattern) {
		if value, ok := attrMap.Get(patternKey); ok {
			return replace(s, pattern, value, logger)
		} else if value, ok := attrMap.Get(patternKeyToAttributeMap[patternKey]); ok {
			return replace(s, pattern, value, logger)
		} else {
			logger.Debug("No resource attribute found for pattern " + pattern)
			return strings.Replace(s, pattern, "undefined", -1)
		}
	}
	return s
}

func replace(s, pattern string, value pdata.AttributeValue, logger *zap.Logger) string {
	if value.StringVal() == "" {
		logger.Debug("Empty resource attribute value found for pattern " + pattern)
		return strings.Replace(s, pattern, "undefined", -1)
	}
	return strings.Replace(s, pattern, value.StringVal(), -1)
}

// getNamespace retrieves namespace for given set of metrics from user config.
func getNamespace(rm *pdata.ResourceMetrics, namespace string) string {
	if len(namespace) == 0 {
		serviceName, svcNameOk := rm.Resource().Attributes().Get(conventions.AttributeServiceName)
		serviceNamespace, svcNsOk := rm.Resource().Attributes().Get(conventions.AttributeServiceNamespace)
		if svcNameOk && svcNsOk && serviceName.Type() == pdata.AttributeValueSTRING && serviceNamespace.Type() == pdata.AttributeValueSTRING {
			namespace = fmt.Sprintf("%s/%s", serviceNamespace.StringVal(), serviceName.StringVal())
		} else if svcNameOk && serviceName.Type() == pdata.AttributeValueSTRING {
			namespace = serviceName.StringVal()
		} else if svcNsOk && serviceNamespace.Type() == pdata.AttributeValueSTRING {
			namespace = serviceNamespace.StringVal()
		}
	}

	if len(namespace) == 0 {
		namespace = defaultNamespace
	}
	return namespace
}

// getLogInfo retrieves the log group and log stream names from a given set of metrics.
func getLogInfo(rm *pdata.ResourceMetrics, cWNamespace string, config *Config) (logGroup, logStream string) {
	if cWNamespace != "" {
		logGroup = fmt.Sprintf("/metrics/%s", cWNamespace)
	}

	// Override log group/stream if specified in config. However, in this case, customer won't have correlation experience
	if len(config.LogGroupName) > 0 {
		logGroup = replacePatterns(config.LogGroupName, rm.Resource().Attributes(), config.logger)
	}
	if len(config.LogStreamName) > 0 {
		logStream = replacePatterns(config.LogStreamName, rm.Resource().Attributes(), config.logger)
	}

	return
}

// unixNanoToMilliseconds converts a timestamp in nanoseconds to milliseconds.
func unixNanoToMilliseconds(timestamp pdata.TimestampUnixNano) int64 {
	return int64(uint64(timestamp) / uint64(time.Millisecond))
}
