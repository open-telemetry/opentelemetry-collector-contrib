// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

var errNilValueProvided = errors.New("nil value provided")

// Manager is responsible for ensuring that schemas are kept up to date
// with the most recent version that are requested.
type Manager interface {
	// RequestTranslation will provide either the defined Translation
	// if it is a known target, or, return a noop variation.
	// In the event that a matched Translation, on a missed version
	// there is a potential to block during this process.
	// Otherwise, the translation will allow concurrent reads.
	RequestTranslation(ctx context.Context, schemaURL string) Translation

	// SetProviders will update the list of providers used by the manager
	// to look up schemaURLs
	SetProviders(providers ...Provider) error
}

type manager struct {
	log *zap.Logger

	rw           sync.RWMutex
	providers    []Provider
	match        map[string]*Version
	translations map[string]*translator
}

var _ Manager = (*manager)(nil)

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
			// todo(ankit) figure out what to do when the providers dont respond something good
		}
		t, err := newTranslatorFromReader(
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
