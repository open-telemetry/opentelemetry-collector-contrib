// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translation

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/alias"
)

// Modifier abstracts a verion change that allows
// for the changes to be updated or revert.
type Modifier interface {
	// UpdateAttrs modifies all matching attributes
	// names with the configured changes.
	UpdateAttrs(attrs pcommon.Map)
	// RevertAttrs modifies all matching attributes
	// names with the configured changes.
	RevertAttrs(attrs pcommon.Map)
	// UpdateAttrsIf only applies the changes configured
	// if the changes explicitly require the match value
	// or if there is no configured values to match against.
	UpdateAttrsIf(match string, attrs pcommon.Map)
	// RevertAttrsIf only applies the changes configured
	// if the changes explicitly require the match value
	// or if there is no configured values to match against.
	RevertAttrsIf(match string, attrs pcommon.Map)
	// UpdateSignal will update the name of the signal
	// if there is a known transition
	UpdateSignal(signal alias.Signal)
	// RevertSignal will update the name of the signal
	// if there is a known transition
	RevertSignal(signal alias.Signal)
}

// modify is one change that can be made to a signal
type modify struct {
	names map[string]string

	appliesTo map[string]struct{}
	attrs     map[string]string
}

// modifications is used when there is sets of changes
// that can not be merged together due to dependant changes
// on signal name
type modifications []*modify

var (
	_ Modifier = (*modify)(nil)
	_ Modifier = (*modifications)(nil)
)

func newModify() *modify {
	return &modify{
		names:     make(map[string]string),
		appliesTo: make(map[string]struct{}),
		attrs:     make(map[string]string),
	}
}

func (mod *modify) UpdateAttrs(attrs pcommon.Map) {
	for from, to := range mod.attrs {
		if v, match := attrs.Get(from); match {
			// Interesting thing of note,
			// ordering of these methods is important
			// otherwise you will get repeated values
			attrs.Insert(to, v)
			attrs.Remove(from)
		}
	}
}

func (mod *modify) RevertAttrs(attrs pcommon.Map) {
	for to, from := range mod.attrs {
		if v, match := attrs.Get(from); match {
			attrs.Insert(to, v)
			attrs.Remove(from)
		}
	}
}

func (mod *modify) UpdateAttrsIf(match string, attrs pcommon.Map) {
	if _, exist := mod.appliesTo[match]; len(mod.appliesTo) != 0 && !exist {
		return
	}
	mod.UpdateAttrs(attrs)
}

func (mod *modify) RevertAttrsIf(match string, attrs pcommon.Map) {
	if _, exist := mod.appliesTo[match]; len(mod.appliesTo) != 0 && !exist {
		return
	}
	mod.RevertAttrs(attrs)
}

func (mod *modify) UpdateSignal(sig alias.Signal) {
	for from, to := range mod.names {
		if from == sig.Name() {
			sig.SetName(to)
			return
		}
	}
}

func (mod *modify) RevertSignal(sig alias.Signal) {
	for to, from := range mod.names {
		if from == sig.Name() {
			sig.SetName(to)
			return
		}
	}
}

func (mods modifications) UpdateAttrs(attrs pcommon.Map) {
	for _, mod := range mods {
		mod.UpdateAttrs(attrs)
	}
}

func (mods modifications) UpdateAttrsIf(match string, attrs pcommon.Map) {
	for _, mod := range mods {
		mod.UpdateAttrsIf(match, attrs)
	}
}

func (mods modifications) UpdateSignal(sig alias.Signal) {
	for _, mod := range mods {
		mod.UpdateSignal(sig)
	}
}

func (mods modifications) RevertAttrs(attrs pcommon.Map) {
	for _, mod := range mods {
		mod.RevertAttrs(attrs)
	}
}

func (mods modifications) RevertAttrsIf(match string, attrs pcommon.Map) {
	for _, mod := range mods {
		mod.RevertAttrsIf(match, attrs)
	}
}

func (mods modifications) RevertSignal(sig alias.Signal) {
	for _, mod := range mods {
		mod.RevertSignal(sig)
	}
}
