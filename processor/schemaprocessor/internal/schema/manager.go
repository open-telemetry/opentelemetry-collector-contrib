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

package schema

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	schemaencoder "go.opentelemetry.io/otel/schema/v1.0"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const (
	DefaultConcurrentFetches = 10
)

var (
	errNilValueProvided = errors.New("nil value provided")
)

// Manager is responsible for ensuring that schemas are kept up to date
// with the most recent version that are requested.
type Manager interface {
	// RequestSchema will returns a schema that can be used to
	// transform attribute values or resource names.
	// In the event that the schemaURL does not exist locally
	// it will enqueued to be fetch and updated.
	// Operations on the returned schema will be blocked
	// until the update is complete
	RequestSchema(schemaURL string) Schema

	// ProcessRequest will perform the network operation
	// of fetching the schema definitions
	// and updating the schema being used.
	ProcessRequests(ctx context.Context) error
}

// ManagerOption is used to include different configurations
// that are considered optional.
type ManagerOption func(*manager) error

// WithManagerLogger will assign the logger to the manager
// provided that it is not nil
func WithManagerLogger(log *zap.Logger) ManagerOption {
	return func(m *manager) error {
		if log == nil {
			return fmt.Errorf("log: %w", errNilValueProvided)
		}
		m.log = log
		return nil
	}
}

// WithManagerHTTPClient will assign the http client
// used by the manager
func WithManagerHTTPClient(c *http.Client) ManagerOption {
	return func(m *manager) error {
		if c == nil {
			return fmt.Errorf("http client: %w", errNilValueProvided)
		}
		m.net = c
		return nil
	}
}

type manager struct {
	net *http.Client
	log *zap.Logger

	mu      sync.Mutex
	reqs    chan string
	schemas map[string]*translation
}

var _ Manager = (*manager)(nil)

// NewManager creates a manager that will allow for management
// of schema, the options allow for additional properties to be
// added to manager to enable additional locations of where to check
// for translations file.
func NewManager(opts ...ManagerOption) (Manager, error) {
	m := &manager{
		net:     &http.Client{},
		log:     zap.NewNop(),
		reqs:    make(chan string, DefaultConcurrentFetches),
		schemas: make(map[string]*translation),
	}

	for _, opt := range opts {
		if err := opt(m); err != nil {
			return nil, err
		}
	}

	return m, nil
}

func (m *manager) RequestSchema(schemaURL string) Schema {
	family, version, err := GetFamilyAndVersion(schemaURL)
	if err != nil {
		m.log.Info("Not a valid schema url was provided, using no-op schema",
			zap.String("schema-url", schemaURL),
		)
		return NoopSchema{}
	}

	m.mu.Lock()
	t, exists := m.schemas[family]
	if !exists {
		t = newTranslation(version)
		m.schemas[family] = t
	}
	m.mu.Unlock()

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

	// Sending request to fetch schema to an async worker to
	// allow for progress until the last possible moment
	m.reqs <- schemaURL

	return t
}

func (m *manager) ProcessRequests(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case schemaURL := <-m.reqs:
			if err := m.GetSchema(ctx, schemaURL); err != nil {
				m.log.Error("Issue trying to fetch schema definition",
					zap.Error(err),
					zap.String("schema-url", schemaURL),
				)
			}
		}

	}
}

func (m *manager) GetSchema(ctx context.Context, schemaURL string) error {
	family, _, err := GetFamilyAndVersion(schemaURL)
	if err != nil {
		return err
	}

	m.mu.Lock()
	t := m.schemas[family]
	m.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, schemaURL, http.NoBody)
	if err != nil {
		return err
	}
	resp, err := m.net.Do(req)
	if err != nil {
		return err
	}

	// checking if 2XX response was returned
	if resp.StatusCode/100 != 2 {
		return multierr.Combine(
			fmt.Errorf("response code: %d", resp.StatusCode),
			resp.Body.Close(),
		)
	}

	t.rw.Lock()
	defer t.rw.Unlock()

	content, err := schemaencoder.Parse(resp.Body)
	if err != nil {
		return multierr.Combine(
			err,
			resp.Body.Close(),
		)
	}

	if err := t.Merge(content); err != nil {
		return multierr.Combine(
			err,
			resp.Body.Close(),
		)
	}

	return resp.Body.Close()
}
