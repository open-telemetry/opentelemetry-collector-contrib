// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchlogsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter"

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

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
	"InstanceId":           "service.instance.id",
	"FaasName":             "faas.name",
	"FaasVersion":          "faas.version",
}

func isPatternValid(s string) (bool, string) {
	if !strings.Contains(s, "{") && !strings.Contains(s, "}") {
		return true, ""
	}

	re := regexp.MustCompile(`\{([^{}]*)\}`)
	matches := re.FindAllStringSubmatch(s, -1)

	for _, match := range matches {
		if len(match) > 1 {
			key := match[1]
			if _, exists := patternKeyToAttributeMap[key]; !exists {
				return false, key
			}
		}
	}
	return true, ""
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
		}
		logger.Debug("No resource attribute found for pattern " + pattern)
		return strings.ReplaceAll(s, pattern, "undefined"), false
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

// getLogInfo retrieves the log group and log stream names from a given set of metrics.
func getLogInfo(resourceAttrs map[string]any, config *Config) (string, string, bool) {
	var logGroup, logStream string
	groupReplaced := true
	streamReplaced := true

	// Convert to map[string]string
	strAttributeMap := anyMapToStringMap(resourceAttrs)

	// Override log group/stream if specified in config. However, in this case, customer won't have correlation experience
	if len(config.LogGroupName) > 0 {
		logGroup, groupReplaced = replacePatterns(config.LogGroupName, strAttributeMap, config.logger)
	}
	if len(config.LogStreamName) > 0 {
		logStream, streamReplaced = replacePatterns(config.LogStreamName, strAttributeMap, config.logger)
	}

	return logGroup, logStream, (groupReplaced && streamReplaced)
}

func anyMapToStringMap(resourceAttrs map[string]any) map[string]string {
	strMap := make(map[string]string)
	for key, value := range resourceAttrs {
		switch v := value.(type) {
		case string:
			strMap[key] = v
		case int:
			strMap[key] = strconv.Itoa(v)
		case bool:
			strMap[key] = strconv.FormatBool(v)
		default:
			strMap[key] = fmt.Sprintf("%v", v)
		}
	}
	return strMap
}
