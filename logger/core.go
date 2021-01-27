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

package logger

import "go.uber.org/zap/zapcore"

// Core is a zap Core used for logging
type Core struct {
	core    zapcore.Core
	emitter *Emitter
}

// With adds contextual fields to the underlying core.
func (c *Core) With(fields []zapcore.Field) zapcore.Core {
	return &Core{
		core:    c.core.With(fields),
		emitter: c.emitter,
	}
}

// Enabled will check if the supplied log level is enabled.
func (c *Core) Enabled(level zapcore.Level) bool {
	return c.core.Enabled(level)
}

// Check checks the entry and determines if the core should write it.
func (c *Core) Check(zapEntry zapcore.Entry, checkedEntry *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if !c.Enabled(zapEntry.Level) {
		return checkedEntry
	}
	return checkedEntry.AddCore(zapEntry, c)
}

// Write sends an entry to the emitter before logging.
func (c *Core) Write(zapEntry zapcore.Entry, fields []zapcore.Field) error {
	stanzaEntry := parseEntry(zapEntry, fields)
	c.emitter.emit(stanzaEntry)
	return c.core.Write(zapEntry, fields)
}

// Sync will sync the underlying core.
func (c *Core) Sync() error {
	return c.core.Sync()
}

// newCore creates a new core.
func newCore(core zapcore.Core, emitter *Emitter) *Core {
	return &Core{
		core:    core,
		emitter: emitter,
	}
}
