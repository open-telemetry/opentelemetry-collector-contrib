// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	// fieldDelimiter is the delimiter used to split a field key into its parts.
	fieldDelimiter = "."

	// fieldEscapeKeyReplacement is the string used to temporarily replace escaped delimters while splitting a field key.
	fieldEscapeKeyReplacement = "{TEMP_REPLACE}"
)

// fieldRemover handles removing excluded fields from log records
type fieldRemover struct {
	fields []*field
}

// field represents a field and it's compound key to match on
type field struct {
	keyParts []string
}

// newFieldRemover creates a new field remover based on the passed in field keys
func newFieldRemover(fieldKeys []string) *fieldRemover {
	fe := &fieldRemover{
		fields: make([]*field, 0, len(fieldKeys)),
	}

	for _, f := range fieldKeys {
		fe.fields = append(fe.fields, &field{
			keyParts: splitField(f),
		})
	}

	return fe
}

// RemoveFields removes any body or attribute fields that match in the log record
func (fe *fieldRemover) RemoveFields(logRecord plog.LogRecord) {
	for _, field := range fe.fields {
		field.removeField(logRecord)
	}
}

// removeField removes the field from the log record if it exists
func (f *field) removeField(logRecord plog.LogRecord) {
	firstPart, remainingParts := f.keyParts[0], f.keyParts[1:]

	switch firstPart {
	case bodyField:
		// If body is a map then recurse through to remove the field
		if logRecord.Body().Type() == pcommon.ValueTypeMap {
			removeFieldFromMap(logRecord.Body().Map(), remainingParts)
		}
	case attributeField:
		// Remove all attributes
		if len(remainingParts) == 0 {
			logRecord.Attributes().Clear()
			return
		}

		// Recurse through map and remove fields
		removeFieldFromMap(logRecord.Attributes(), remainingParts)
	}
}

// removeFieldFromMap recurses through the map and removes the field if it's found.
func removeFieldFromMap(valueMap pcommon.Map, keyParts []string) {
	nextKeyPart, remainingParts := keyParts[0], keyParts[1:]

	// Look for the value associated with the next key part.
	// If we don't find it then return
	value, ok := valueMap.Get(nextKeyPart)
	if !ok {
		return
	}

	// No more key parts that means we have found the value and remove it
	if len(remainingParts) == 0 {
		valueMap.Remove(nextKeyPart)
		return
	}

	// If the value is a map then recurse through with the remaining parts
	if value.Type() == pcommon.ValueTypeMap {
		removeFieldFromMap(value.Map(), remainingParts)
	}
}

// splitField splits a field key into its parts.
// It replaces escaped delimiters with the full delimiter after splitting.
func splitField(fieldKey string) []string {
	escapedKey := strings.ReplaceAll(fieldKey, fmt.Sprintf("\\%s", fieldDelimiter), fieldEscapeKeyReplacement)
	keyParts := strings.Split(escapedKey, fieldDelimiter)

	// Replace the temporarily escaped delimiters with the actual delimiter.
	for i := range keyParts {
		keyParts[i] = strings.ReplaceAll(keyParts[i], fieldEscapeKeyReplacement, fieldDelimiter)
	}

	return keyParts
}
