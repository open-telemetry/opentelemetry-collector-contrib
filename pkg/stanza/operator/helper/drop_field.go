// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"errors"
	"fmt"
	"slices"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// DropFieldConfig provides configuration for dropping parsed fields from an entry.
type DropFieldConfig struct {
	DropField bool `mapstructure:"drop_field,omitempty"`
}

// ValidateDropField checks that parseFrom can be safely dropped when DropField is true.
func (c DropFieldConfig) ValidateDropField(parseFrom entry.Field) error {
	if !c.DropField {
		return nil
	}
	prefix, keys, ok := fieldPrefixAndKeys(parseFrom)
	// If we determine that this we are requesting the drop of a root key
	// (`body`, `attributes`, `resource`) we will fail, as we disallow
	// dropping those fields from the entry.
	if ok && len(keys) == 0 {
		return fmt.Errorf("`drop_field: true` cannot be used when `parse_from` is `%s`", prefix)
	}
	return nil
}

// ValidateDropFieldWithTarget checks that parseFrom can be safely dropped and does not
// equal or contain parseTo when DropField is true.
func (c DropFieldConfig) ValidateDropFieldWithTarget(parseFrom, parseTo entry.Field) error {
	if err := c.ValidateDropField(parseFrom); err != nil {
		return err
	}
	if !c.DropField {
		return nil
	}
	fromPrefix, fromKeys, fromOK := fieldPrefixAndKeys(parseFrom)
	toPrefix, toKeys, toOK := fieldPrefixAndKeys(parseTo)
	// This will check whether `parse_to` is the same as, or a subfield of, `parse_from`. For the two being
	// equal, the issue is obvious (it will just remove the data that we want to keep). If `parse_to` is a
	// subfield, then removing `parse_from` will inadvertently drop the newly parsed data as well.
	if fromOK && toOK && fromPrefix == toPrefix && len(toKeys) >= len(fromKeys) && slices.Equal(fromKeys, toKeys[:len(fromKeys)]) {
		if len(toKeys) == len(fromKeys) {
			return errors.New("`parse_to` and `parse_from` cannot be the same when `drop_field: true`")
		}
		return errors.New("`parse_to` cannot be a subfield of `parse_from` when `drop_field: true`")
	}
	return nil
}

func fieldPrefixAndKeys(field entry.Field) (string, []string, bool) {
	switch f := field.FieldInterface.(type) {
	case entry.BodyField:
		return entry.BodyPrefix, f.Keys, true
	case entry.AttributeField:
		return entry.AttributesPrefix, f.Keys, true
	case entry.ResourceField:
		return entry.ResourcePrefix, f.Keys, true
	default:
		return "", nil, false
	}
}

// Drop deletes the field from the entry if DropField is set to true.
func (c DropFieldConfig) Drop(ent *entry.Entry, field entry.Field) {
	if c.DropField {
		ent.Delete(field)
	}
}
