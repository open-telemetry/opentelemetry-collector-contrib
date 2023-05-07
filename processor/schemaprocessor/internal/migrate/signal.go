// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package migrate // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/migrate"

import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"

// SignalType allows for type constraints in order
// to apply to potential type defined strings.
type SignalType interface {
	~string
}

// Signal allows for migrating types that
// implement the `alias.Signal` interface.
type Signal struct {
	updates  map[string]string
	rollback map[string]string
}

type SignalSlice []*Signal

// NewSignal will create a `Signal` that will check the provided mappings if it can update a `alias.Signal`
// and if no values are provided for `matches`, then all values will be updated.
func NewSignal[Key SignalType, Value SignalType](mappings map[Key]Value) *Signal {
	sig := &Signal{
		updates:  make(map[string]string, len(mappings)),
		rollback: make(map[string]string, len(mappings)),
	}
	for k, v := range mappings {
		sig.updates[string(k)] = string(v)
		sig.rollback[string(v)] = string(k)
	}
	return sig
}

func (s *Signal) Apply(signal alias.Signal) {
	s.do(StateSelctorApply, signal)
}

func (s *Signal) Rollback(signal alias.Signal) {
	s.do(StateSelectorRollback, signal)
}

func (s *Signal) do(ss StateSelctor, signal alias.Signal) {
	var (
		name    string
		matched bool
	)
	switch ss {
	case StateSelctorApply:
		name, matched = s.updates[signal.Name()]
	case StateSelectorRollback:
		name, matched = s.rollback[signal.Name()]
	}
	if matched {
		signal.SetName(name)
	}
}

func NewSignalSlice(changes ...*Signal) *SignalSlice {
	values := new(SignalSlice)
	for _, c := range changes {
		(*values) = append((*values), c)
	}
	return values
}

func (slice *SignalSlice) Apply(signal alias.Signal) {
	slice.do(StateSelctorApply, signal)
}

func (slice *SignalSlice) Rollback(signal alias.Signal) {
	slice.do(StateSelectorRollback, signal)
}

func (slice *SignalSlice) do(ss StateSelctor, signal alias.Signal) {
	for i := 0; i < len((*slice)); i++ {
		switch ss {
		case StateSelctorApply:
			(*slice)[i].Apply(signal)
		case StateSelectorRollback:
			(*slice)[len((*slice))-i-1].Rollback(signal)
		}
	}
}
