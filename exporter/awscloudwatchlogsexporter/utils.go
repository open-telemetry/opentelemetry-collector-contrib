package awscloudwatchlogsexporter

import (
	"go.uber.org/zap"
	"strings"
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
