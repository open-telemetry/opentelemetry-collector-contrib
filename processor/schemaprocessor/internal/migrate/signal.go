// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package migrate // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"

import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"

// SignalType allows for type constraints in order
// to apply to potential type defined strings.
type SignalType interface {
	~string
}

// SignalNameChange allows for migrating types that
// implement the `alias.NamedSignal` interface.
type SignalNameChange struct {
	updates  map[string]string
	rollback map[string]string
}

// NewSignalNameChange will create a `Signal` that will check the provided mappings if it can update a `alias.NamedSignal`
// and if no values are provided for `matches`, then all values will be updated.
func NewSignalNameChange[Key SignalType, Value SignalType](mappings map[Key]Value) SignalNameChange {
	sig := SignalNameChange{
		updates:  make(map[string]string, len(mappings)),
		rollback: make(map[string]string, len(mappings)),
	}
	for k, v := range mappings {
		sig.updates[string(k)] = string(v)
		sig.rollback[string(v)] = string(k)
	}
	return sig
}

func (s SignalNameChange) IsMigrator() {}

func (s *SignalNameChange) Do(ss StateSelector, signal alias.NamedSignal) {
	var (
		name    string
		matched bool
	)
	switch ss {
	case StateSelectorApply:
		name, matched = s.updates[signal.Name()]
	case StateSelectorRollback:
		name, matched = s.rollback[signal.Name()]
	}
	if matched {
		signal.SetName(name)
	}
}
