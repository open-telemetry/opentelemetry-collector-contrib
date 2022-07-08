package cwlogs

import (
	"go.uber.org/zap"
	"strings"
)

func ReplacePatterns(s string, attrMap map[string]string, logger *zap.Logger) (string, bool) {
	success := true
	var foundAndReplaced bool
	for key := range PatternKeyToAttributeMap {
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
		} else if value, ok := attrMap[PatternKeyToAttributeMap[patternKey]]; ok {
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
