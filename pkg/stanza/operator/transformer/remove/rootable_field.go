// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remove // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/remove"

import (
	"encoding/json"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// RootableField represents a potential field on an entry.
// It differs from a normal Field in that it allows users to
// specify `resource` or `attributes` with the intention
// of referring to "all" fields within those groups.
// It is used to get, set, and delete values at this field.
// It is deserialized from JSON dot notation.
type rootableField struct {
	entry.Field
	allResource   bool
	allAttributes bool
}

// UnmarshalJSON will unmarshal a field from JSON
func (f *rootableField) UnmarshalJSON(raw []byte) error {
	var s string
	err := json.Unmarshal(raw, &s)
	if err != nil {
		return err
	}
	return f.unmarshalCheckString(s)
}

// UnmarshalYAML will unmarshal a field from YAML
func (f *rootableField) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return err
	}
	return f.unmarshalCheckString(s)
}

// UnmarshalText will unmarshal a field from text
func (f *rootableField) UnmarshalText(text []byte) error {
	return f.unmarshalCheckString(string(text))
}

func (f *rootableField) unmarshalCheckString(s string) error {
	if s == entry.ResourcePrefix {
		*f = rootableField{allResource: true}
		return nil
	}

	if s == entry.AttributesPrefix {
		*f = rootableField{allAttributes: true}
		return nil
	}

	field, err := entry.NewField(s)
	if err != nil {
		return err
	}
	*f = rootableField{Field: field}
	return nil
}

// Get gets the value of the field if the flags for 'allAttributes' or 'allResource' isn't set
func (f *rootableField) Get(entry *entry.Entry) (interface{}, bool) {
	if f.allAttributes || f.allResource {
		return nil, false
	}
	return f.Field.Get(entry)
}
