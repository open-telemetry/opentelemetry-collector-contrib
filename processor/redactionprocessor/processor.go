// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"

//nolint:gosec
import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash"
	"regexp"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"golang.org/x/crypto/sha3"
)

const attrValuesSeparator = ","

type redaction struct {
	// Attribute keys allowed in a span
	allowList map[string]string
	// Attribute keys ignored in a span
	ignoreList map[string]string
	// Attribute values blocked in a span
	blockRegexList map[string]*regexp.Regexp
	// Attribute values allowed in a span
	allowRegexList map[string]*regexp.Regexp
	// Attribute keys blocked in a span
	blockKeyRegexList map[string]*regexp.Regexp
	// Hash function to hash blocked values
	hashFunction HashFunction
	// Redaction processor configuration
	config *Config
	// Logger
	logger *zap.Logger
}

// newRedaction creates a new instance of the redaction processor
func newRedaction(ctx context.Context, config *Config, logger *zap.Logger) (*redaction, error) {
	allowList := makeAllowList(config)
	ignoreList := makeIgnoreList(config)
	blockRegexList, err := makeRegexList(ctx, config.BlockedValues)
	if err != nil {
		// TODO: Placeholder for an error metric in the next PR
		return nil, fmt.Errorf("failed to process block list: %w", err)
	}
	blockKeysRegexList, err := makeRegexList(ctx, config.BlockedKeyPatterns)
	if err != nil {
		// TODO: Placeholder for an error metric in the next PR
		return nil, fmt.Errorf("failed to process block keys list: %w", err)
	}

	allowRegexList, err := makeRegexList(ctx, config.AllowedValues)
	if err != nil {
		// TODO: Placeholder for an error metric in the next PR
		return nil, fmt.Errorf("failed to process allow list: %w", err)
	}

	return &redaction{
		allowList:         allowList,
		ignoreList:        ignoreList,
		blockRegexList:    blockRegexList,
		allowRegexList:    allowRegexList,
		blockKeyRegexList: blockKeysRegexList,
		hashFunction:      config.HashFunction,
		config:            config,
		logger:            logger,
	}, nil
}

// processTraces implements ProcessMetricsFunc. It processes the incoming data
// and returns the data to be sent to the next component
func (s *redaction) processTraces(ctx context.Context, batch ptrace.Traces) (ptrace.Traces, error) {
	for i := 0; i < batch.ResourceSpans().Len(); i++ {
		rs := batch.ResourceSpans().At(i)
		s.processResourceSpan(ctx, rs)
	}
	return batch, nil
}

func (s *redaction) processLogs(ctx context.Context, logs plog.Logs) (plog.Logs, error) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		s.processResourceLog(ctx, rl)
	}
	return logs, nil
}

func (s *redaction) processMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		s.processResourceMetric(ctx, rm)
	}
	return metrics, nil
}

// processResourceSpan processes the RS and all of its spans and then returns the last
// view metric context. The context can be used for tests
func (s *redaction) processResourceSpan(ctx context.Context, rs ptrace.ResourceSpans) {
	rsAttrs := rs.Resource().Attributes()

	// Attributes can be part of a resource span
	s.processAttrs(ctx, rsAttrs)

	for j := 0; j < rs.ScopeSpans().Len(); j++ {
		ils := rs.ScopeSpans().At(j)
		for k := 0; k < ils.Spans().Len(); k++ {
			span := ils.Spans().At(k)
			spanAttrs := span.Attributes()

			// Attributes can also be part of span
			s.processAttrs(ctx, spanAttrs)

			// Attributes can also be part of span events
			s.processSpanEvents(ctx, span.Events())
		}
	}
}

func (s *redaction) processSpanEvents(ctx context.Context, events ptrace.SpanEventSlice) {
	for i := 0; i < events.Len(); i++ {
		s.processAttrs(ctx, events.At(i).Attributes())
	}
}

// processResourceLog processes the log resource and all of its logs and then returns the last
// view metric context. The context can be used for tests
func (s *redaction) processResourceLog(ctx context.Context, rl plog.ResourceLogs) {
	rsAttrs := rl.Resource().Attributes()

	s.processAttrs(ctx, rsAttrs)

	for j := 0; j < rl.ScopeLogs().Len(); j++ {
		ils := rl.ScopeLogs().At(j)
		for k := 0; k < ils.LogRecords().Len(); k++ {
			log := ils.LogRecords().At(k)
			s.processAttrs(ctx, log.Attributes())
			s.processLogBody(ctx, log.Body(), log.Attributes())
		}
	}
}

func (s *redaction) processLogBody(ctx context.Context, body pcommon.Value, attributes pcommon.Map) {
	var redactedKeys, maskedKeys, allowedKeys, ignoredKeys []string

	switch body.Type() {
	case pcommon.ValueTypeMap:
		var redactedBodyKeys []string
		body.Map().Range(func(k string, v pcommon.Value) bool {
			if s.shouldIgnoreKey(k) {
				ignoredKeys = append(ignoredKeys, k)
				return true
			}
			if s.shouldRedactKey(k) {
				redactedBodyKeys = append(redactedBodyKeys, k)
				return true
			}
			if s.shouldMaskKey(k) {
				maskedKeys = append(maskedKeys, k)
				v.SetStr(s.maskValue(v.Str(), regexp.MustCompile(".*")))
				return true
			}
			s.redactLogBodyRecursive(ctx, k, v, &redactedKeys, &maskedKeys, &allowedKeys, &ignoredKeys)
			return true
		})
		for _, k := range redactedBodyKeys {
			body.Map().Remove(k)
			redactedKeys = append(redactedKeys, k)
		}
	case pcommon.ValueTypeSlice:
		for i := 0; i < body.Slice().Len(); i++ {
			s.redactLogBodyRecursive(ctx, fmt.Sprintf("[%d]", i), body.Slice().At(i), &redactedKeys, &maskedKeys, &allowedKeys, &ignoredKeys)
		}
	default:
		strVal := body.AsString()
		if s.shouldAllowValue(strVal) {
			allowedKeys = append(allowedKeys, "body")
			return
		}
		processedValue := s.processStringValue(strVal)
		if strVal != processedValue {
			maskedKeys = append(maskedKeys, "body")
			body.SetStr(processedValue)
		}
	}

	s.addMetaAttrs(redactedKeys, attributes, redactionBodyRedactedKeys, redactionBodyRedactedCount)
	s.addMetaAttrs(maskedKeys, attributes, redactionBodyMaskedKeys, redactionBodyMaskedCount)
	s.addMetaAttrs(allowedKeys, attributes, redactionBodyAllowedKeys, redactionBodyAllowedCount)
	s.addMetaAttrs(ignoredKeys, attributes, "", redactionBodyIgnoredCount)
}

func (s *redaction) redactLogBodyRecursive(ctx context.Context, key string, value pcommon.Value, redactedKeys, maskedKeys, allowedKeys, ignoredKeys *[]string) {
	switch value.Type() {
	case pcommon.ValueTypeMap:
		var redactedCurrentValueKeys []string
		value.Map().Range(func(k string, v pcommon.Value) bool {
			keyWithPath := fmt.Sprintf("%s.%s", key, k)
			if s.shouldIgnoreKey(k) {
				*ignoredKeys = append(*ignoredKeys, keyWithPath)
				return true
			}
			if s.shouldRedactKey(k) {
				redactedCurrentValueKeys = append(redactedCurrentValueKeys, k)
				return true
			}
			if s.shouldMaskKey(k) {
				*maskedKeys = append(*maskedKeys, keyWithPath)
				v.SetStr(s.maskValue(v.Str(), regexp.MustCompile(".*")))
				return true
			}
			s.redactLogBodyRecursive(ctx, keyWithPath, v, redactedKeys, maskedKeys, allowedKeys, ignoredKeys)
			return true
		})
		for _, k := range redactedCurrentValueKeys {
			value.Map().Remove(k)
			keyWithPath := fmt.Sprintf("%s.%s", key, k)
			*redactedKeys = append(*redactedKeys, keyWithPath)
		}
	case pcommon.ValueTypeSlice:
		for i := 0; i < value.Slice().Len(); i++ {
			keyWithPath := fmt.Sprintf("%s.[%d]", key, i)
			s.redactLogBodyRecursive(ctx, keyWithPath, value.Slice().At(i), redactedKeys, maskedKeys, allowedKeys, ignoredKeys)
		}
	default:
		strVal := value.AsString()
		if s.shouldAllowValue(strVal) {
			*allowedKeys = append(*allowedKeys, key)
			return
		}
		processedValue := s.processStringValue(strVal)
		if strVal != processedValue {
			*maskedKeys = append(*maskedKeys, key)
			value.SetStr(processedValue)
		}
	}
}

func (s *redaction) processResourceMetric(ctx context.Context, rm pmetric.ResourceMetrics) {
	rsAttrs := rm.Resource().Attributes()

	s.processAttrs(ctx, rsAttrs)

	for j := 0; j < rm.ScopeMetrics().Len(); j++ {
		ils := rm.ScopeMetrics().At(j)
		for k := 0; k < ils.Metrics().Len(); k++ {
			metric := ils.Metrics().At(k)
			switch metric.Type() {
			case pmetric.MetricTypeGauge:
				dps := metric.Gauge().DataPoints()
				for i := 0; i < dps.Len(); i++ {
					s.processAttrs(ctx, dps.At(i).Attributes())
				}
			case pmetric.MetricTypeSum:
				dps := metric.Sum().DataPoints()
				for i := 0; i < dps.Len(); i++ {
					s.processAttrs(ctx, dps.At(i).Attributes())
				}
			case pmetric.MetricTypeHistogram:
				dps := metric.Histogram().DataPoints()
				for i := 0; i < dps.Len(); i++ {
					s.processAttrs(ctx, dps.At(i).Attributes())
				}
			case pmetric.MetricTypeExponentialHistogram:
				dps := metric.ExponentialHistogram().DataPoints()
				for i := 0; i < dps.Len(); i++ {
					s.processAttrs(ctx, dps.At(i).Attributes())
				}
			case pmetric.MetricTypeSummary:
				dps := metric.Summary().DataPoints()
				for i := 0; i < dps.Len(); i++ {
					s.processAttrs(ctx, dps.At(i).Attributes())
				}
			case pmetric.MetricTypeEmpty:
			}
		}
	}
}

// processAttrs redacts the attributes of a resource span or a span
func (s *redaction) processAttrs(_ context.Context, attributes pcommon.Map) {
	// TODO: Use the context for recording metrics
	var redactedKeys, maskedKeys, allowedKeys, ignoredKeys []string

	// Identify attributes to redact and mask in the following sequence
	// 1. Make a list of attribute keys to redact
	// 2. Mask any blocked values for the other attributes
	// 3. Delete the attributes from 1
	//
	// This sequence satisfies these performance constraints:
	// - Only range through all attributes once
	// - Don't mask any values if the whole attribute is slated for deletion
AttributeLoop:
	for k, value := range attributes.All() {
		if s.shouldIgnoreKey(k) {
			ignoredKeys = append(ignoredKeys, k)
			continue AttributeLoop
		}
		if s.shouldRedactKey(k) {
			redactedKeys = append(redactedKeys, k)
			continue AttributeLoop
		}
		strVal := value.Str()
		if s.shouldAllowValue(strVal) {
			allowedKeys = append(allowedKeys, k)
			continue AttributeLoop
		}
		if s.shouldMaskKey(k) {
			maskedKeys = append(maskedKeys, k)
			maskedValue := s.maskValue(strVal, regexp.MustCompile(".*"))
			value.SetStr(maskedValue)
			continue AttributeLoop
		}
		processedString := s.processStringValue(strVal)
		if processedString != strVal {
			maskedKeys = append(maskedKeys, k)
			value.SetStr(processedString)
		}
	}

	// Delete the attributes on the redaction list
	for _, k := range redactedKeys {
		attributes.Remove(k)
	}
	// Add diagnostic information to the span
	s.addMetaAttrs(redactedKeys, attributes, redactionRedactedKeys, redactionRedactedCount)
	s.addMetaAttrs(maskedKeys, attributes, redactionMaskedKeys, redactionMaskedCount)
	s.addMetaAttrs(allowedKeys, attributes, redactionAllowedKeys, redactionAllowedCount)
	s.addMetaAttrs(ignoredKeys, attributes, "", redactionIgnoredCount)
}

//nolint:gosec
func (s *redaction) maskValue(val string, regex *regexp.Regexp) string {
	hashFunc := func(match string) string {
		switch s.hashFunction {
		case SHA1:
			return hashString(match, sha1.New())
		case SHA3:
			return hashString(match, sha3.New256())
		case MD5:
			return hashString(match, md5.New())
		default:
			return "****"
		}
	}
	return regex.ReplaceAllStringFunc(val, hashFunc)
}

func hashString(input string, hasher hash.Hash) string {
	hasher.Write([]byte(input))
	return hex.EncodeToString(hasher.Sum(nil))
}

// addMetaAttrs adds diagnostic information about redacted or masked attribute keys
func (s *redaction) addMetaAttrs(redactedAttrs []string, attributes pcommon.Map, valuesAttr, countAttr string) {
	redactedCount := int64(len(redactedAttrs))
	if redactedCount == 0 {
		return
	}

	// Record summary as span attributes, empty string for ignored items
	if s.config.Summary == debug && len(valuesAttr) > 0 {
		if existingVal, found := attributes.Get(valuesAttr); found && existingVal.Str() != "" {
			redactedAttrs = append(redactedAttrs, strings.Split(existingVal.Str(), attrValuesSeparator)...)
		}
		sort.Strings(redactedAttrs)
		attributes.PutStr(valuesAttr, strings.Join(redactedAttrs, attrValuesSeparator))
	}
	if s.config.Summary == info || s.config.Summary == debug {
		if existingVal, found := attributes.Get(countAttr); found {
			redactedCount += existingVal.Int()
		}
		attributes.PutInt(countAttr, redactedCount)
	}
}

func (s *redaction) processStringValue(strVal string) string {
	// Mask any blocked values for the other attributes
	for _, compiledRE := range s.blockRegexList {
		match := compiledRE.MatchString(strVal)
		if match {
			strVal = s.maskValue(strVal, compiledRE)
		}
	}
	return strVal
}

func (s *redaction) shouldMaskKey(k string) bool {
	// Mask any blocked keys for the other attributes
	for _, compiledRE := range s.blockKeyRegexList {
		if match := compiledRE.MatchString(k); match {
			return true
		}
	}
	return false
}

func (s *redaction) shouldAllowValue(strVal string) bool {
	// Allow any values matching the allowed list regex
	for _, compiledRE := range s.allowRegexList {
		if match := compiledRE.MatchString(strVal); match {
			return true
		}
	}
	return false
}

func (s *redaction) shouldIgnoreKey(k string) bool {
	if _, ignored := s.ignoreList[k]; ignored {
		return true
	}
	return false
}

func (s *redaction) shouldRedactKey(k string) bool {
	if !s.config.AllowAllKeys {
		if _, found := s.allowList[k]; !found {
			return true
		}
	}
	return false
}

const (
	debug                      = "debug"
	info                       = "info"
	redactionRedactedKeys      = "redaction.redacted.keys"
	redactionRedactedCount     = "redaction.redacted.count"
	redactionMaskedKeys        = "redaction.masked.keys"
	redactionMaskedCount       = "redaction.masked.count"
	redactionAllowedKeys       = "redaction.allowed.keys"
	redactionAllowedCount      = "redaction.allowed.count"
	redactionIgnoredCount      = "redaction.ignored.count"
	redactionBodyRedactedKeys  = "redaction.body.redacted.keys"
	redactionBodyRedactedCount = "redaction.body.redacted.count"
	redactionBodyMaskedKeys    = "redaction.body.masked.keys"
	redactionBodyMaskedCount   = "redaction.body.masked.count"
	redactionBodyAllowedKeys   = "redaction.body.allowed.keys"
	redactionBodyAllowedCount  = "redaction.body.allowed.count"
	redactionBodyIgnoredCount  = "redaction.body.ignored.count"
)

// makeAllowList sets up a lookup table of allowed span attribute keys
func makeAllowList(c *Config) map[string]string {
	// redactionKeys are additional span attributes created by the processor to
	// summarize the changes it made to a span. If the processor removes
	// 2 attributes from a span (e.g. `birth_date`, `mothers_maiden_name`),
	// then it will list them in the `redaction.redacted.keys` span attribute
	// and set the `redaction.redacted.count` attribute to 2
	//
	// If the processor finds and masks values matching a blocked regex in 2
	// span attributes (e.g. `notes`, `description`), then it will those
	// attribute keys in `redaction.masked.keys` and set the
	// `redaction.masked.count` to 2
	redactionKeys := []string{redactionRedactedKeys, redactionRedactedCount, redactionMaskedKeys, redactionMaskedCount, redactionIgnoredCount}
	// allowList consists of the keys explicitly allowed by the configuration
	// as well as of the new span attributes that the processor creates to
	// summarize its changes
	allowList := make(map[string]string, len(c.AllowedKeys)+len(redactionKeys))
	for _, key := range c.AllowedKeys {
		allowList[key] = key
	}
	for _, key := range redactionKeys {
		allowList[key] = key
	}
	return allowList
}

func makeIgnoreList(c *Config) map[string]string {
	ignoreList := make(map[string]string, len(c.IgnoredKeys))
	for _, key := range c.IgnoredKeys {
		ignoreList[key] = key
	}
	return ignoreList
}

// makeRegexList precompiles all the regex patterns in the defined list
func makeRegexList(_ context.Context, valuesList []string) (map[string]*regexp.Regexp, error) {
	regexList := make(map[string]*regexp.Regexp, len(valuesList))
	for _, pattern := range valuesList {
		re, err := regexp.Compile(pattern)
		if err != nil {
			// TODO: Placeholder for an error metric in the next PR
			return nil, fmt.Errorf("error compiling regex in list: %w", err)
		}
		regexList[pattern] = re
	}
	return regexList, nil
}
