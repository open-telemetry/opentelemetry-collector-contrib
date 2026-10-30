// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver"

import (
	"sync"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/config/configdbauth"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
)

// dbAuthResolver resolves a configured credential provider when the Collector
// host is available at Start. Resolution is shared across all query scrapers in
// a receiver instance.
type dbAuthResolver struct {
	id configdbauth.ID

	once     sync.Once
	provider dbauth.Provider
	err      error
}

func newDBAuthResolver(id configdbauth.ID) *dbAuthResolver {
	return &dbAuthResolver{id: id}
}

func (r *dbAuthResolver) resolve(host component.Host) error {
	if r == nil || r.id.IsEmpty() {
		return nil
	}
	r.once.Do(func() {
		r.provider, r.err = r.id.GetProvider(host.GetExtensions())
	})
	return r.err
}

func (r *dbAuthResolver) getProvider() dbauth.Provider {
	if r == nil {
		return nil
	}
	return r.provider
}
