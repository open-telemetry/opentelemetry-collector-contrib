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

		SetProviders(providers ...Provider) error
	}

	manager struct {
		log *zap.Logger

		rw           sync.RWMutex
		providers    []Provider
		match        map[string]*Version
		translations map[string]*translator
	}

	updateRequest struct {
		// ctx is stored here so that it can
		// cancel a background routine early.
		ctx        context.Context
		schemaURL  string
		translator *translator
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

	match := make(map[string]*Version, len(targets))
	for _, target := range targets {
		family, version, err := GetFamilyAndVersion(target)
		if err != nil {
			return nil, err
		}
		match[family] = version
	}

	return &manager{
		log:          log,
		match:        match,
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

	target, match := m.match[family]
	if !match {
		m.log.Debug("Not a known target, providing Nop Translation",
			zap.String("schema-url", schemaURL),
		)
		return nopTranslation{}
	}

	m.rw.RLock()
	t, exists := m.translations[family]
	m.rw.RUnlock()

	if exists && t.SupportedVersion(version) {
		return t
	}

	for _, p := range m.providers {
		content, err := p.Lookup(ctx, schemaURL)
		if err != nil {
			m.log.Error("Failed to lookup schemaURL",
				zap.Error(err),
				zap.String("schemaURL", schemaURL),
			)
		}
		t, err := newTranslater(
			m.log.Named("translator").With(
				zap.String("family", family),
				zap.Stringer("target", target),
			),
			joinSchemaFamilyAndVersion(family, target),
			content,
		)
		if err != nil {
			m.log.Error("Failed to create translator", zap.Error(err))
			continue
		}
		m.rw.Lock()
		m.translations[family] = t
		m.rw.Unlock()
		return t
	}

	return nopTranslation{}
}

func (m *manager) SetProviders(providers ...Provider) error {
	if len(providers) == 0 {
		return fmt.Errorf("zero providers set: %w", errNilValueProvided)
	}
	m.rw.Lock()
	m.providers = append(m.providers[:0], providers...)
	m.rw.Unlock()
	return nil
}
