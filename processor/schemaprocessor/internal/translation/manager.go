// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

var errNilValueProvided = errors.New("nil value provided")

// Manager is responsible for ensuring that schemas are kept up to date
// with the most recent version that are requested.
type Manager interface {
	// RequestTranslation will provide either the defined Translation
	// if it is a known target and able to retrieve from Provider
	// otherwise it will return an error.
	RequestTranslation(ctx context.Context, schemaURL string) (Translation, error)

	// AddProvider will add a provider to the Manager
	AddProvider(p Provider)
}

type manager struct {
	log *zap.Logger

	rw            sync.RWMutex
	providers     []Provider
	match         map[string]*Version
	translatorMap map[string]*translator
}

var _ Manager = (*manager)(nil)

// NewManager creates a manager that will allow for management
// of schema
func NewManager(targetSchemaURLS []string, log *zap.Logger, providers ...Provider) (Manager, error) {
	if log == nil {
		return nil, fmt.Errorf("logger: %w", errNilValueProvided)
	}

	match := make(map[string]*Version, len(targetSchemaURLS))
	for _, target := range targetSchemaURLS {
		family, version, err := GetFamilyAndVersion(target)
		if err != nil {
			return nil, err
		}
		match[family] = version
	}

	// wrap provider with cacheable provider
	var prs []Provider
	for _, p := range providers {
		// TODO make cache configurable
		prs = append(prs, NewCacheableProvider(p, 5*time.Minute, 5))
	}

	return &manager{
		log:           log,
		match:         match,
		translatorMap: make(map[string]*translator),
		providers:     prs,
	}, nil
}

func (m *manager) RequestTranslation(ctx context.Context, schemaURL string) (Translation, error) {
	m.log.Debug("Requesting translation for schemaURL", zap.String("schema-url", schemaURL))
	family, version, err := GetFamilyAndVersion(schemaURL)
	if err != nil {
		m.log.Error("No valid schema url was provided",
			zap.String("schema-url", schemaURL), zap.Error(err),
		)
		return nil, err
	}

	targetTranslation, match := m.match[family]
	if !match {
		m.log.Warn("Not a known target translation",
			zap.String("schema-url", schemaURL),
		)
		return nil, fmt.Errorf("not a known targetTranslation: %s", family)
	}

	m.rw.RLock()
	t, exists := m.translatorMap[family]
	m.rw.RUnlock()

	if exists && t.SupportedVersion(version) {
		return t, nil
	}

	for _, p := range m.providers {
		content, err := p.Retrieve(ctx, schemaURL)
		if err != nil {
			m.log.Error("Failed to lookup schemaURL",
				zap.Error(err),
				zap.String("schemaURL", schemaURL),
			)
			// If we fail to retrieve the schema, we should
			// try the next provider
			continue
		}
		t, err := newTranslator(
			m.log.Named("translator").With(
				zap.String("family", family),
				zap.Stringer("target", targetTranslation),
			),
			joinSchemaFamilyAndVersion(family, targetTranslation),
			content,
		)
		if err != nil {
			m.log.Error("Failed to create translator", zap.Error(err))
			continue
		}
		m.rw.Lock()
		m.translatorMap[family] = t
		m.rw.Unlock()
		return t, nil
	}

	return nil, fmt.Errorf("failed to retrieve translation for %s", schemaURL)
}

// AddProvider will add a provider to the Manager
func (m *manager) AddProvider(p Provider) {
	if p == nil {
		m.log.Error("Nil provider provided, not adding to manager")
		return
	}
	if _, ok := p.(*CacheableProvider); !ok {
		p = NewCacheableProvider(p, 5*time.Minute, 5)
	}
	m.providers = append(m.providers, p)
}
