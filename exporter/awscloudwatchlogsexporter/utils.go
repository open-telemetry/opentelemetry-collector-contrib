// Copyright  The OpenTelemetry Authors
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

package awscloudwatchlogsexporter

import (
	"strings"

	"go.uber.org/zap"
)

func attrsString(attrs map[string]interface{}) map[string]string {
	out := make(map[string]string, len(attrs))
	for k, s := range attrs {
		out[k] = s.(string)
	}
	return out
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

func getLogInfo(rm *cwLogBody, config *Config) (string, string, bool) {
	var logGroup, logStream string
	groupReplaced := true
	streamReplaced := true

	strAttributeMap := attrsString(rm.Resource)

	// Override log group/stream if specified in config. However, in this case, customer won't have correlation experience
	if len(config.LogGroupName) > 0 {
		logGroup, groupReplaced = replacePatterns(config.LogGroupName, strAttributeMap, config.logger)
	}
	if len(config.LogStreamName) > 0 {
		logStream, streamReplaced = replacePatterns(config.LogStreamName, strAttributeMap, config.logger)
	}

	return logGroup, logStream, (groupReplaced && streamReplaced)
}
