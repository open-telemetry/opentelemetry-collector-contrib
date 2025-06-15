// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"

// NoopField is a struct that implements Field, but
// does nothing for all its operations.
type NoopField struct{}

// Get will return always return nil
func (l NoopField) Get(_ *Entry) (any, bool) {
	return nil, true
}

// Set will do nothing and return no error
func (l NoopField) Set(_ *Entry, _ any) error {
	return nil
}

// Delete will do nothing and return no error
func (l NoopField) Delete(_ *Entry) (any, bool) {
	return nil, true
}

func (l NoopField) String() string {
	return "$nil"
}

// NewNoopField will create a new noop field.
func NewNoopField() Field {
	return Field{NoopField{}}
}

// NewNilField will create a new nil field. This
// is used to facilitate nil checks in config
// validation.
func NewNilField() Field {
	return Field{}
}
