// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension"

import (
	"context"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

const jwksCacheTTL = 5 * time.Minute

type expiringRemoteKeySet struct {
	ctx context.Context
	url string
	now func() time.Time

	mu        sync.Mutex
	keySet    *oidc.RemoteKeySet
	expiresAt time.Time
}

func newExpiringRemoteKeySet(ctx context.Context, url string) *expiringRemoteKeySet {
	return &expiringRemoteKeySet{
		ctx: context.WithoutCancel(ctx), url: url, now: time.Now,
	}
}

func (s *expiringRemoteKeySet) VerifySignature(ctx context.Context, token string) ([]byte, error) {
	s.mu.Lock()
	now := s.now()
	if s.keySet == nil || !now.Before(s.expiresAt) {
		// go-oidc only refreshes on a cache miss. Replace its entire cache on
		// expiry so a known, removed key cannot keep authenticating forever.
		s.keySet = oidc.NewRemoteKeySet(s.ctx, s.url)
		s.expiresAt = now.Add(jwksCacheTTL)
	}
	keySet := s.keySet
	s.mu.Unlock()

	// Network I/O stays outside the lock; go-oidc coordinates concurrent fetches.
	// An expired cache is never used as a fallback when the new fetch fails.
	return keySet.VerifySignature(ctx, token)
}
