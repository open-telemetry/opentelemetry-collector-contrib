// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"

// NilField is a struct that implements Field, but
// does nothing for all its operations. It is useful
// as a default no-op field to avoid nil checks.
//
// Deprecated: Originally used for empty Field comparisons,
// use Field.IsEmpty instead.
type NilField struct{}

// Get will return always return nil
func (NilField) Get(*Entry) (any, bool) {
	return nil, true
}

// Set will do nothing and return no error
func (NilField) Set(*Entry, any) error {
	return nil
}

// Delete will do nothing and return no error
func (NilField) Delete(*Entry) (any, bool) {
	return nil, true
}

func (NilField) String() string {
	return "$nil"
}

// NewNilField will create a new nil field
func NewNilField() Field {
	return Field{FieldInterface: NilField{}}
}
