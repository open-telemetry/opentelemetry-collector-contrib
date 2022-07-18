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

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import "go.opentelemetry.io/otel/schema/v1.0/ast"

// Revision represents all changes that are to be
// applied to a signal at a given version.
type Revision interface {
	// Version is stored to help make
	// sorting revisions easier
	Version() *Version

	// All is intended to apply to all incoming attributes
	// that are known as part of this revision.
	All() Modifier

	// Resources applies to all resources
	// of the incoming data
	Resources() Modifier

	// Logs applies to all incoming log signals that
	// are part of this revision
	Logs() Modifier

	// Spans update all incoming span signals
	// that are part of this revision
	Spans() Modifier

	// SpanEvents update all incoming span event signals
	// that are part of this revision
	SpanEvents() Modifier

	// Metrics update all incoming metric signals
	// that are part of this reivision
	Metrics() Modifier
}

type revision struct {
	ver      *Version
	all      Modifier
	resource Modifier
	logs     Modifier
	spans    Modifier
	events   Modifier
	metrics  Modifier
}

var (
	_ Revision = (*revision)(nil)
)

// newRevision processes the VersionDef and assigns the version to this revision
// to allow sorting within a slice.
// Since VersionDef uses custom types for various definitions, it isn't possible
// to cast those values into the primitives so each has to be processed together.
// Generics would be handy here.
func newRevision(ver *Version, def ast.VersionDef) Revision {
	r := &revision{
		ver:      ver,
		all:      noModify{},
		resource: noModify{},
		logs:     noModify{},
		spans:    noModify{},
		events:   noModify{},
		metrics:  noModify{},
	}

	if len(def.All.Changes) > 0 {
		mod := newModify()
		for _, ch := range def.All.Changes {
			if ch.RenameAttributes == nil {
				continue
			}
			for k, v := range *ch.RenameAttributes {
				mod.attrs[k] = v
			}
		}
		r.all = mod
	}

	if len(def.Resources.Changes) > 0 {
		mod := newModify()
		for _, ch := range def.Resources.Changes {
			if ch.RenameAttributes == nil {
				continue
			}
			for k, v := range *ch.RenameAttributes {
				mod.attrs[k] = v
			}
		}
		r.resource = mod
	}

	if l := len(def.Logs.Changes); l > 0 {
		mod := newModify()
		for _, ch := range def.Logs.Changes {
			if ch.RenameAttributes == nil {
				continue
			}
			for k, v := range ch.RenameAttributes.AttributeMap {
				mod.attrs[k] = v
			}
		}
		r.logs = mod
	}

	if l := len(def.Metrics.Changes); l > 0 {
		mods := make(modifications, l)
		for i, ch := range def.Metrics.Changes {
			mod := newModify()
			if ch.RenameAttributes != nil {
				for _, name := range ch.RenameAttributes.ApplyToMetrics {
					mod.appliesTo[string(name)] = struct{}{}
				}
				for k, v := range ch.RenameAttributes.AttributeMap {
					mod.attrs[k] = v
				}
			}
			for k, v := range ch.RenameMetrics {
				mod.names[string(k)] = string(v)
			}
			mods[i] = mod
		}
		r.metrics = mods
	}
	if l := len(def.Spans.Changes); l > 0 {
		mods := make(modifications, l)
		for i, ch := range def.Spans.Changes {
			mod := newModify()
			if ch.RenameAttributes != nil {
				for _, name := range ch.RenameAttributes.ApplyToSpans {
					mod.appliesTo[string(name)] = struct{}{}
				}
				for k, v := range ch.RenameAttributes.AttributeMap {
					mod.attrs[k] = v
				}
			}
			mods[i] = mod
		}
		r.spans = mods
	}

	if l := len(def.SpanEvents.Changes); l > 0 {
		mods := make(modifications, l)
		for i, ch := range def.SpanEvents.Changes {
			mod := newModify()
			if ch.RenameAttributes != nil {
				for _, name := range ch.RenameAttributes.ApplyToEvents {
					mod.appliesTo[string(name)] = struct{}{}
				}
				for _, name := range ch.RenameAttributes.ApplyToSpans {
					mod.appliesTo[string(name)] = struct{}{}
				}
				for k, v := range ch.RenameAttributes.AttributeMap {
					mod.attrs[k] = v
				}
			}
			if ch.RenameEvents != nil {
				for k, v := range ch.RenameEvents.EventNameMap {
					mod.names[k] = v
				}
			}
			mods[i] = mod
		}
		r.events = mods
	}

	return r
}

func (r *revision) Version() *Version    { return r.ver }
func (r *revision) All() Modifier        { return r.all }
func (r *revision) Resources() Modifier  { return r.resource }
func (r *revision) Logs() Modifier       { return r.logs }
func (r *revision) Spans() Modifier      { return r.spans }
func (r *revision) SpanEvents() Modifier { return r.events }
func (r *revision) Metrics() Modifier    { return r.metrics }
