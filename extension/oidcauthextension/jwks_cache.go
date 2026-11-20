// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-jose/go-jose/v4"
	"go.uber.org/zap"
)

const (
	jwksCacheTTL        = 5 * time.Minute
	jwksRefreshInterval = 4 * time.Minute
	jwksRetryInterval   = 15 * time.Second
)

type jwksSnapshot struct {
	keys      []jose.JSONWebKey
	expiresAt time.Time
}

type jwksRefresh struct {
	done chan struct{}
	err  error
}

type expiringRemoteKeySet struct {
	ctx      context.Context
	cancel   context.CancelFunc
	url      string
	client   *http.Client
	logger   *zap.Logger
	now      func() time.Time
	snapshot atomic.Pointer[jwksSnapshot]
	ready    chan struct{}
	mu       sync.Mutex
	inflight *jwksRefresh
	stopped  bool
	wg       sync.WaitGroup
}

func newExpiringRemoteKeySet(ctx context.Context, url string, client *http.Client, logger *zap.Logger) *expiringRemoteKeySet {
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s := &expiringRemoteKeySet{ctx: ctx, cancel: cancel, url: url, client: client, logger: logger, now: time.Now, ready: make(chan struct{})}
	s.wg.Add(1)
	go s.run()
	return s
}

func (s *expiringRemoteKeySet) stop() {
	s.mu.Lock()
	s.stopped = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *expiringRemoteKeySet) run() {
	defer s.wg.Done()
	select {
	case <-s.ctx.Done():
		return
	case <-s.ready:
	}
	timer := time.NewTimer(jwksRefreshInterval)
	defer timer.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-timer.C:
		}
		err := s.refresh(s.ctx)
		if s.ctx.Err() != nil {
			return
		}
		delay := jwksRefreshInterval
		if err != nil {
			s.logger.Warn("Failed to refresh remote JWKS", zap.Error(err))
			delay = jwksRetryInterval
		}
		timer.Reset(delay)
	}
}

func (s *expiringRemoteKeySet) VerifySignature(ctx context.Context, token string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if snapshot := s.snapshot.Load(); snapshot != nil && s.now().Before(snapshot.expiresAt) {
		if payload, err := verifyJWKS(snapshot.keys, token); err == nil {
			return payload, nil
		}
	}
	// Preserve immediate initial loading and key rotation on a cache miss.
	// Regular refreshes run ahead of expiry without blocking cached requests.
	if err := s.refresh(ctx); err != nil {
		return nil, err
	}
	snapshot := s.snapshot.Load()
	if snapshot == nil || !s.now().Before(snapshot.expiresAt) {
		return nil, errors.New("JWKS cache expired")
	}
	return verifyJWKS(snapshot.keys, token)
}

func (s *expiringRemoteKeySet) refresh(ctx context.Context) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return context.Canceled
	}
	call := s.inflight
	if call == nil {
		call = &jwksRefresh{done: make(chan struct{})}
		s.inflight = call
		s.wg.Add(1)
		go s.fetchAndPublish(call)
	}
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-call.done:
		return call.err
	}
}

func (s *expiringRemoteKeySet) fetchAndPublish(call *jwksRefresh) {
	defer s.wg.Done()
	keys, err := s.fetch()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped && err == nil {
		err = context.Canceled
	}
	if err == nil {
		// A failed refresh must never renew the old snapshot's trust lifetime.
		initial := s.snapshot.Load() == nil
		s.snapshot.Store(&jwksSnapshot{keys: keys, expiresAt: s.now().Add(jwksCacheTTL)})
		if initial {
			close(s.ready)
		}
	}
	call.err = err
	s.inflight = nil
	close(call.done)
}

func (s *expiringRemoteKeySet) fetch() ([]jose.JSONWebKey, error) {
	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, s.url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var set struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("decoding JWKS: %w", err)
	}
	var keys []jose.JSONWebKey
	for _, raw := range set.Keys {
		var metadata struct {
			Alg string `json:"alg"`
		}
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return nil, err
		}
		if metadata.Alg != "" && !slices.Contains(allSupportedAlgorithms, jose.SignatureAlgorithm(metadata.Alg)) {
			continue
		}
		var key jose.JSONWebKey
		if err := json.Unmarshal(raw, &key); err != nil {
			if errors.Is(err, jose.ErrUnsupportedKeyType) {
				continue
			}
			return nil, err
		}
		if key.IsPublic() && key.Valid() {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil, errors.New("JWKS contains no supported public keys")
	}
	return keys, nil
}

func verifyJWKS(keys []jose.JSONWebKey, token string) ([]byte, error) {
	signed, err := jose.ParseSignedCompact(token, allSupportedAlgorithms)
	if err != nil {
		return nil, err
	}
	keyID := signed.Signatures[0].Header.KeyID
	for i := range keys {
		if keyID == "" || keys[i].KeyID == keyID {
			if payload, err := signed.Verify(&keys[i]); err == nil {
				return payload, nil
			}
		}
	}
	return nil, errors.New("failed to verify id token signature")
}
