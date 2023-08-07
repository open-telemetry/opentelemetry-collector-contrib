// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"

// NilField is a struct that implements Field, but
// does nothing for all its operations. It is useful
// as a default no-op field to avoid nil checks.
type NilField struct{}

// Get will return always return nil
func (l NilField) Get(_ *Entry) (interface{}, bool) {
	return nil, true
}

// Set will do nothing and return no error
func (l NilField) Set(_ *Entry, _ interface{}) error {
	return nil
}

// Delete will do nothing and return no error
func (l NilField) Delete(_ *Entry) (interface{}, bool) {
	return nil, true
}

func (l NilField) String() string {
	return "$nil"
}

// NewNilField will create a new nil field
func NewNilField() Field {
	return Field{NilField{}}
}
