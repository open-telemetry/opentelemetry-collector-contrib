// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

const attrValuesSeparator = ","

type redaction struct {
	// Attribute keys allowed in a span
	allowList map[string]string
	// Attribute keys ignored in a span
	ignoreList map[string]string
	// Attribute values blocked in a span
	blockRegexList map[string]*regexp.Regexp
	// Redaction processor configuration
	config *Config
	// Logger
	logger *zap.Logger
}

// newRedaction creates a new instance of the redaction processor
func newRedaction(ctx context.Context, config *Config, logger *zap.Logger) (*redaction, error) {
	allowList := makeAllowList(config)
	ignoreList := makeIgnoreList(config)
	blockRegexList, err := makeBlockRegexList(ctx, config)
	if err != nil {
		// TODO: Placeholder for an error metric in the next PR
		return nil, fmt.Errorf("failed to process block list: %w", err)
	}

	return &redaction{
		allowList:      allowList,
		ignoreList:     ignoreList,
		blockRegexList: blockRegexList,
		config:         config,
		logger:         logger,
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
		}
	}
}

// processAttrs redacts the attributes of a resource span or a span
func (s *redaction) processAttrs(_ context.Context, attributes pcommon.Map) {
	// TODO: Use the context for recording metrics
	var toDelete []string
	var toBlock []string
	var ignoring []string

	// Identify attributes to redact and mask in the following sequence
	// 1. Make a list of attribute keys to redact
	// 2. Mask any blocked values for the other attributes
	// 3. Delete the attributes from 1
	//
	// This sequence satisfies these performance constraints:
	// - Only range through all attributes once
	// - Don't mask any values if the whole attribute is slated for deletion
	attributes.Range(func(k string, value pcommon.Value) bool {
		// don't delete or redact the attribute if it should be ignored
		if _, ignored := s.ignoreList[k]; ignored {
			ignoring = append(ignoring, k)
			// Skip to the next attribute
			return true
		}

		// Make a list of attribute keys to redact
		if !s.config.AllowAllKeys {
			if _, allowed := s.allowList[k]; !allowed {
				toDelete = append(toDelete, k)
				// Skip to the next attribute
				return true
			}
		}

		// Mask any blocked values for the other attributes
		strVal := value.Str()
		for _, compiledRE := range s.blockRegexList {
			match := compiledRE.MatchString(strVal)
			if match {
				toBlock = append(toBlock, k)

				maskedValue := compiledRE.ReplaceAllString(strVal, "****")
				value.SetStr(maskedValue)
			}
		}
		return true
	})

	// Delete the attributes on the redaction list
	for _, k := range toDelete {
		attributes.Remove(k)
	}
	// Add diagnostic information to the span
	s.addMetaAttrs(toDelete, attributes, redactedKeys, redactedKeyCount)
	s.addMetaAttrs(toBlock, attributes, maskedValues, maskedValueCount)
	s.addMetaAttrs(ignoring, attributes, "", ignoredKeyCount)
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

const (
	debug            = "debug"
	info             = "info"
	redactedKeys     = "redaction.redacted.keys"
	redactedKeyCount = "redaction.redacted.count"
	maskedValues     = "redaction.masked.keys"
	maskedValueCount = "redaction.masked.count"
	ignoredKeyCount  = "redaction.ignored.count"
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
	redactionKeys := []string{redactedKeys, redactedKeyCount, maskedValues, maskedValueCount, ignoredKeyCount}
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

// makeBlockRegexList precompiles all the blocked regex patterns
func makeBlockRegexList(_ context.Context, config *Config) (map[string]*regexp.Regexp, error) {
	blockRegexList := make(map[string]*regexp.Regexp, len(config.BlockedValues))
	for _, pattern := range config.BlockedValues {
		re, err := regexp.Compile(pattern)
		if err != nil {
			// TODO: Placeholder for an error metric in the next PR
			return nil, fmt.Errorf("error compiling regex in block list: %w", err)
		}
		blockRegexList[pattern] = re
	}
	return blockRegexList, nil
}
