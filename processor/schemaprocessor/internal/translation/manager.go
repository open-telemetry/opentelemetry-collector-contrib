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

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

var (
	errNilValueProvided = errors.New("nil value provided")
)

type (
	// Manager is responsible for ensuring that schemas are kept up to date
	// with the most recent version that are requested.
	Manager interface {
		// RequestTranslation will provide either the defined Translation
		// if it is a known target, or, return a noop variation.
		// In the event that a matched Translation, on a missed version
		// there is a potential to block during this process.
		// Otherwise, the translation will allow concurrent reads.
		RequestTranslation(ctx context.Context, schemaURL string) Translation

		// Start is intended to be run within its own go routine
		// so that it can provide updates asynchronisely.
		// The providers will be checked in order provided.
		Run(ctx context.Context, providers ...Provider) error
	}

	manager struct {
		log *zap.Logger

		reqs chan *updateRequest

		rw           sync.RWMutex
		match        map[string]struct{}
		translations map[string]*translator
	}

	updateRequest struct {
		// ctx is stored here so that it can
		// cancel a background routine early.
		ctx        context.Context
		schemaURL  string
		translater *translator
	}
)

var (
	_ Manager = (*manager)(nil)
)

// NewManager creates a manager that will allow for management
// of schema, the options allow for additional properties to be
// added to manager to enable additional locations of where to check
// for translations file.
func NewManager(targets []string, log *zap.Logger) (Manager, error) {
	if log == nil {
		return nil, fmt.Errorf("logger: %w", errNilValueProvided)
	}

	match := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		family, _, err := GetFamilyAndVersion(target)
		if err != nil {
			return nil, err
		}
		match[family] = struct{}{}
	}

	return &manager{
		log:          log,
		match:        match,
		reqs:         make(chan *updateRequest, 10),
		translations: make(map[string]*translator),
	}, nil
}

func (m *manager) RequestTranslation(ctx context.Context, schemaURL string) Translation {
	family, version, err := GetFamilyAndVersion(schemaURL)
	if err != nil {
		m.log.Debug("No valid schema url was provided, using no-op schema",
			zap.String("schema-url", schemaURL),
		)
		return nopTranslation{}
	}

	if _, targeting := m.match[family]; !targeting {
		m.log.Debug("Not a known target, providing Nop Translation",
			zap.String("schema-url", schemaURL),
		)
		return nopTranslation{}
	}

	m.rw.RLock()
	t, exists := m.translations[family]
	m.rw.RUnlock()

	if !exists {
		m.rw.Lock()
		t, err = newTranslater(
			m.log.Named("translation").With(
				zap.String("family", family),
				zap.Stringer("target", version),
			),
			schemaURL,
		)
		if err != nil {
			m.log.Error("Issue trying to create translater", zap.Error(err))
			m.rw.Unlock()
			return nopTranslation{}
		}
		m.translations[family] = t
		m.rw.Unlock()
	}

	// In the event that this schema has already been fetched
	// and requests a version that is already supported
	// the cached instance of it is then returned
	if exists && t.SupportedVersion(version) {
		m.log.Debug("Using cached version of schema",
			zap.String("family", family),
			zap.Stringer("version", version),
		)
		return t
	}

	// Locking the translater now so that any
	// reads are blocked until the update
	// happens or once the context is cancelled.
	// This ensures that applying updates are consistent
	// and simplifies locking processes
	t.rw.Lock()
	m.reqs <- &updateRequest{
		ctx:        ctx,
		translater: t,
		schemaURL:  schemaURL,
	}

	return t
}

func (m *manager) Run(ctx context.Context, providers ...Provider) error {
	if len(providers) == 0 {
		return fmt.Errorf("zero providers set: %w", errNilValueProvided)
	}
	for {
		select {
		case <-ctx.Done():
			m.log.Debug("Manager is shutting down", zap.Error(ctx.Err()))
			return nil
		case req := <-m.reqs:
			for _, p := range providers {
				content, err := p.Lookup(req.ctx, req.schemaURL)
				if err != nil {
					m.log.Error("Issue with looking up schemaURL", zap.Error(err))
					continue
				}
				if content == nil {
					m.log.Debug("SchemaURL not present in lookup")
					continue
				}
				if err := req.translater.merge(content); err != nil {
					m.log.Error("Issue trying to update translater", zap.Error(err))
					continue
				}
				break
			}
			req.translater.rw.Unlock()
		}
	}
}
