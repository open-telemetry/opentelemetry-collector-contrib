// Copyright The OpenTelemetry Authors
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

package awsemfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

var patternKeyToAttributeMap = map[string]string{
	"ClusterName":          "aws.ecs.cluster.name",
	"TaskId":               "aws.ecs.task.id",
	"NodeName":             "k8s.node.name",
	"PodName":              "pod",
	"ServiceName":          "service.name",
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
			return strings.ReplaceAll(s, pattern, "undefined"), false
		}
	}
	return s, true
}

func replace(s, pattern string, value string, logger *zap.Logger) (string, bool) {
	if value == "" {
		logger.Debug("Empty resource attribute value found for pattern " + pattern)
		return strings.ReplaceAll(s, pattern, "undefined"), false
	}
	return strings.ReplaceAll(s, pattern, value), true
}

// getNamespace retrieves namespace for given set of metrics from user config.
func getNamespace(rm pmetric.ResourceMetrics, namespace string) string {
	if len(namespace) == 0 {
		serviceName, svcNameOk := rm.Resource().Attributes().Get(conventions.AttributeServiceName)
		serviceNamespace, svcNsOk := rm.Resource().Attributes().Get(conventions.AttributeServiceNamespace)
		switch {
		case svcNameOk && svcNsOk && serviceName.Type() == pcommon.ValueTypeStr && serviceNamespace.Type() == pcommon.ValueTypeStr:
			namespace = fmt.Sprintf("%s/%s", serviceNamespace.Str(), serviceName.Str())
		case svcNameOk && serviceName.Type() == pcommon.ValueTypeStr:
			namespace = serviceName.Str()
		case svcNsOk && serviceNamespace.Type() == pcommon.ValueTypeStr:
			namespace = serviceNamespace.Str()
		}
	}

	if len(namespace) == 0 {
		namespace = defaultNamespace
	}
	return namespace
}

// getLogInfo retrieves the log group and log stream names from a given set of metrics.
func getLogInfo(rm pmetric.ResourceMetrics, cWNamespace string, config *Config) (string, string, bool) {
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

	// Empty dimension must be always present in a roll up.
	dimensionZero := []string{}

	instrLibName, hasOTelKey := labels[oTellibDimensionKey]
	if hasOTelKey {
		// If OTel key exists in labels, add it as a zero dimension but remove it
		// temporarily from labels as it is not an original label
		dimensionZero = append(dimensionZero, oTellibDimensionKey)
		delete(labels, oTellibDimensionKey)
	}

	if dimensionRollupOption == zeroAndSingleDimensionRollup {
		// "Zero" dimension rollup
		if len(labels) > 0 {
			rollupDimensionArray = append(rollupDimensionArray, dimensionZero)
		}
	}
	if dimensionRollupOption == zeroAndSingleDimensionRollup || dimensionRollupOption == singleDimensionRollupOnly {
		// "One" dimension rollup
		for labelName := range labels {
			dimSet := dimensionZero
			dimSet = append(dimSet, labelName)
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
func unixNanoToMilliseconds(timestamp pcommon.Timestamp) int64 {
	return int64(uint64(timestamp) / uint64(time.Millisecond))
}

// attrMaptoStringMap converts a pcommon.Map to a map[string]string
func attrMaptoStringMap(attrMap pcommon.Map) map[string]string {
	strMap := make(map[string]string, attrMap.Len())

	attrMap.Range(func(k string, v pcommon.Value) bool {
		strMap[k] = v.AsString()
		return true
	})
	return strMap
}
