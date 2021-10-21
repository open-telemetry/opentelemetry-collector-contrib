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
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"
)

var patternKeyToAttributeMap = map[string]string{
	"ClusterName":          "aws.ecs.cluster.name",
	"TaskId":               "aws.ecs.task.id",
	"NodeName":             "k8s.node.name",
	"PodName":              "pod",
	"ContainerInstanceId":  "aws.ecs.container.instance.id",
	"TaskDefinitionFamily": "aws.ecs.task.family",
}

func replacePatterns(s string, attrMap map[string]string, logger *zap.Logger) (string, bool) {
	success := true
	var foundAndReplaced bool
	for key := range patternKeyToAttributeMap {
		s, foundAndReplaced = replacePatternWithAttrValue(s, key, attrMap, logger)
		success = success && foundAndReplaced
	}
	return s, success
}

func replacePatternWithAttrValue(s, patternKey string, attrMap map[string]string, logger *zap.Logger) (string, bool) {
	pattern := "{" + patternKey + "}"
	if strings.Contains(s, pattern) {
		if value, ok := attrMap[patternKey]; ok {
			return replace(s, pattern, value, logger)
		} else if value, ok := attrMap[patternKeyToAttributeMap[patternKey]]; ok {
			return replace(s, pattern, value, logger)
		} else {
			logger.Debug("No resource attribute found for pattern " + pattern)
			return strings.Replace(s, pattern, "undefined", -1), false
		}
	}
	return s, true
}

func replace(s, pattern string, value string, logger *zap.Logger) (string, bool) {
	if value == "" {
		logger.Debug("Empty resource attribute value found for pattern " + pattern)
		return strings.Replace(s, pattern, "undefined", -1), false
	}
	return strings.Replace(s, pattern, value, -1), true
}

// getNamespace retrieves namespace for given set of metrics from user config.
func getNamespace(rm *pdata.ResourceMetrics, namespace string) string {
	if len(namespace) == 0 {
		serviceName, svcNameOk := rm.Resource().Attributes().Get(conventions.AttributeServiceName)
		serviceNamespace, svcNsOk := rm.Resource().Attributes().Get(conventions.AttributeServiceNamespace)
		if svcNameOk && svcNsOk && serviceName.Type() == pdata.AttributeValueTypeString && serviceNamespace.Type() == pdata.AttributeValueTypeString {
			namespace = fmt.Sprintf("%s/%s", serviceNamespace.StringVal(), serviceName.StringVal())
		} else if svcNameOk && serviceName.Type() == pdata.AttributeValueTypeString {
			namespace = serviceName.StringVal()
		} else if svcNsOk && serviceNamespace.Type() == pdata.AttributeValueTypeString {
			namespace = serviceNamespace.StringVal()
		}
	}

	if len(namespace) == 0 {
		namespace = defaultNamespace
	}
	return namespace
}

// getLogInfo retrieves the log group and log stream names from a given set of metrics.
func getLogInfo(rm *pdata.ResourceMetrics, cWNamespace string, config *Config) (string, string, bool) {
	var logGroup, logStream string
	groupReplaced := true
	streamReplaced := true

	if cWNamespace != "" {
		logGroup = fmt.Sprintf("/metrics/%s", cWNamespace)
	}

	strAttributeMap := attrMaptoStringMap(rm.Resource().Attributes())

	// Override log group/stream if specified in config. However, in this case, customer won't have correlation experience
	if len(config.LogGroupName) > 0 {
		logGroup, groupReplaced = replacePatterns(config.LogGroupName, strAttributeMap, config.logger)
	}
	if len(config.LogStreamName) > 0 {
		logStream, streamReplaced = replacePatterns(config.LogStreamName, strAttributeMap, config.logger)
	}

	return logGroup, logStream, (groupReplaced && streamReplaced)
}

// dedupDimensions removes duplicated dimension sets from the given dimensions.
// Prerequisite: each dimension set is already sorted
func dedupDimensions(dimensions [][]string) (deduped [][]string) {
	seen := make(map[string]bool)
	for _, dimSet := range dimensions {
		key := strings.Join(dimSet, ",")
		// Only add dimension set if not a duplicate
		if _, ok := seen[key]; !ok {
			deduped = append(deduped, dimSet)
			seen[key] = true
		}
	}
	return
}

// dimensionRollup creates rolled-up dimensions from the metric's label set.
// The returned dimensions are sorted in alphabetical order within each dimension set
func dimensionRollup(dimensionRollupOption string, labels map[string]string) [][]string {
	var rollupDimensionArray [][]string
	dimensionZero := make([]string, 0)

	instrLibName, hasOTelKey := labels[oTellibDimensionKey]
	if hasOTelKey {
		// If OTel key exists in labels, add it as a zero dimension but remove it
		// temporarily from labels as it is not an original label
		dimensionZero = []string{oTellibDimensionKey}
		delete(labels, oTellibDimensionKey)
	}

	if dimensionRollupOption == zeroAndSingleDimensionRollup {
		//"Zero" dimension rollup
		if len(labels) > 0 {
			rollupDimensionArray = append(rollupDimensionArray, dimensionZero)
		}
	}
	if dimensionRollupOption == zeroAndSingleDimensionRollup || dimensionRollupOption == singleDimensionRollupOnly {
		//"One" dimension rollup
		for labelName := range labels {
			dimSet := append(dimensionZero, labelName)
			sort.Strings(dimSet)
			rollupDimensionArray = append(rollupDimensionArray, dimSet)
		}
	}

	// Add back OTel key to labels if it was removed
	if hasOTelKey {
		labels[oTellibDimensionKey] = instrLibName
	}

	return rollupDimensionArray
}

// unixNanoToMilliseconds converts a timestamp in nanoseconds to milliseconds.
func unixNanoToMilliseconds(timestamp pdata.Timestamp) int64 {
	return int64(uint64(timestamp) / uint64(time.Millisecond))
}

// attrMaptoStringMap converts a pdata.AttributeMap to a map[string]string
func attrMaptoStringMap(attrMap pdata.AttributeMap) map[string]string {
	strMap := make(map[string]string, attrMap.Len())

	attrMap.Range(func(k string, v pdata.AttributeValue) bool {
		strMap[k] = v.AsString()
		return true
	})
	return strMap
}
