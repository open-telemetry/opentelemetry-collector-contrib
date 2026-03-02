// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/fsnotify/fsnotify"
	"github.com/go-jose/go-jose/v4"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var (
	_ extension.Extension  = (*oidcExtension)(nil)
	_ extensionauth.Server = (*oidcExtension)(nil)
)

type providerContainer struct {
	providerCfg ProviderCfg
	provider    *oidc.Provider
	verifier    *oidc.IDTokenVerifier
	verifierMu  sync.RWMutex // protects verifier for hot-reload
	verifierCfg *oidc.Config // stored for recreating verifier on reload
	client      *http.Client
	transport   *http.Transport
}

func (pc *providerContainer) Verify(ctx context.Context, raw string) (*oidc.IDToken, error) {
	var verifier *oidc.IDTokenVerifier

	// No need to grab the lock, we aren't hot-reloading from a file.
	if pc.providerCfg.PublicKeysFile == "" {
		verifier = pc.verifier
	} else {
		pc.verifierMu.RLock()
		verifier = pc.verifier
		pc.verifierMu.RUnlock()
	}

	return verifier.Verify(ctx, raw)
}

func (pc *providerContainer) refreshPublicKeysFile() error {
	publicKeys, err := parseJWKSFile(pc.providerCfg.PublicKeysFile)
	if err != nil {
		return err
	}

	keySet := &oidc.StaticKeySet{}
	supportedAlgs := []string{}
	for _, pk := range publicKeys {
		keySet.PublicKeys = append(keySet.PublicKeys, pk.publicKey)
		for _, a := range pk.supportedAlgorithms {
			// jose.SignatureAlgorithm actually is a string.
			supportedAlgs = append(supportedAlgs, string(a))
		}
	}

	pc.verifierMu.Lock()
	pc.verifierCfg.SupportedSigningAlgs = supportedAlgs
	pc.verifier = oidc.NewVerifier(pc.providerCfg.IssuerURL, keySet, pc.verifierCfg)
	pc.verifierMu.Unlock()

	return nil
}

type oidcExtension struct {
	cfg *Config

	providerContainers map[string]*providerContainer
	logger             *zap.Logger
	shutdownCH         chan struct{}
	shutdownOnce       sync.Once
}

var (
	errNoAudienceProvided                = errors.New("no Audience provided for the OIDC configuration")
	errNoIssuerURL                       = errors.New("no IssuerURL provided for the OIDC configuration")
	errInvalidAuthenticationHeaderFormat = errors.New("invalid authorization header format")
	errFailedToObtainClaimsFromToken     = errors.New("failed to get the subject from the token issued by the OIDC provider")
	errClaimNotFound                     = errors.New("username claim from the OIDC configuration not found on the token returned by the OIDC provider")
	errUsernameNotString                 = errors.New("the username returned by the OIDC provider isn't a regular string")
	errGroupsClaimNotFound               = errors.New("groups claim from the OIDC configuration not found on the token returned by the OIDC provider")
	errNotAuthenticated                  = errors.New("authentication didn't succeed")
	errNoSupportedKeys                   = errors.New("file contains no supported keys (supported types are RSA, ECDSA, and ED25519)")
)

func newExtension(cfg *Config, logger *zap.Logger) extension.Extension {
	if cfg.Attribute == "" {
		cfg.Attribute = defaultAttribute
	}

	return &oidcExtension{
		cfg:                cfg,
		logger:             logger,
		providerContainers: make(map[string]*providerContainer),
	}
}

func (e *oidcExtension) Start(ctx context.Context, _ component.Host) error {
	var errs error
	for _, providerCfg := range e.cfg.getProviderConfigs() {
		errs = multierr.Append(errs, e.processProviderConfig(ctx, providerCfg))
	}
	if errs != nil {
		return fmt.Errorf("failed to get configuration from at least one configured auth server: %w", errs)
	}

	// Start file watchers for providers with JWKS files
	if err := e.startFileWatchers(ctx); err != nil {
		return fmt.Errorf("failed to start file watchers: %w", err)
	}

	return nil
}

func (e *oidcExtension) Shutdown(context.Context) error {
	// Signal file watchers to stop
	e.shutdownOnce.Do(func() {
		if e.shutdownCH != nil {
			close(e.shutdownCH)
		}
	})

	for _, p := range e.providerContainers {
		if p.client != nil {
			p.client.CloseIdleConnections()
		}
		if p.transport != nil {
			p.transport.CloseIdleConnections()
		}
	}

	return nil
}

// authenticate checks whether the given context contains valid auth data. Successfully authenticated calls will always return a nil error and a context with the auth data.
func (e *oidcExtension) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	var authHeaders []string
	for k, v := range headers {
		if strings.EqualFold(k, e.cfg.Attribute) {
			authHeaders = v
			break
		}
	}
	if len(authHeaders) == 0 {
		return ctx, errNotAuthenticated
	}

	// we only use the first header, if multiple values exist
	parts := strings.Split(authHeaders[0], " ")
	if len(parts) != 2 {
		return ctx, errInvalidAuthenticationHeaderFormat
	}

	raw := parts[1]
	unverifiedIssuer, err := getIssuerFromUnverifiedJWT(raw)
	if err != nil {
		return ctx, fmt.Errorf("failed to parse the token: %w", err)
	}
	pc, err := e.resolveProvider(unverifiedIssuer)
	if err != nil {
		return ctx, fmt.Errorf("failed to resolve OIDC provider for the issuer %q: %w", unverifiedIssuer, err)
	}

	idToken, err := pc.Verify(ctx, raw)
	if err != nil {
		return ctx, fmt.Errorf("failed to verify token: %w", err)
	}

	claims := map[string]any{}
	if err = idToken.Claims(&claims); err != nil {
		// currently, this isn't a valid condition, the Verify call a few lines above
		// will already attempt to parse the payload as a json and set it as the claims
		// for the token. As we are using a map to hold the claims, there's no way to fail
		// to read the claims. It could fail if we were using a custom struct. Instead of
		// swallowing the error, it's better to make this future-proof, in case the underlying
		// code changes
		return ctx, errFailedToObtainClaimsFromToken
	}

	subject, err := getSubjectFromClaims(claims, pc.providerCfg.UsernameClaim, idToken.Subject)
	if err != nil {
		return ctx, fmt.Errorf("failed to get subject from claims in the token: %w", err)
	}
	membership, err := getGroupsFromClaims(claims, pc.providerCfg.GroupsClaim)
	if err != nil {
		return ctx, fmt.Errorf("failed to get groups from claims in the token: %w", err)
	}

	cl := client.FromContext(ctx)
	cl.Auth = &authData{
		raw:        raw,
		claims:     claims,
		subject:    subject,
		membership: membership,
	}
	return client.NewContext(ctx, cl), nil
}

func (e *oidcExtension) resolveProvider(issuer string) (*providerContainer, error) {
	if len(e.providerContainers) == 1 {
		for _, pc := range e.providerContainers {
			return pc, nil
		}
	}
	pc, ok := e.providerContainers[issuer]
	if !ok {
		return nil, fmt.Errorf("no OIDC provider configured for the issuer %q", issuer)
	}
	return pc, nil
}

func (e *oidcExtension) processProviderConfig(ctx context.Context, p ProviderCfg) error {
	pc := providerContainer{
		providerCfg: p,
	}

	vCfg := &oidc.Config{
		ClientID:          p.Audience,
		SkipClientIDCheck: p.IgnoreAudience,
	}

	if p.PublicKeysFile != "" {
		pc.verifierCfg = vCfg
		if err := pc.refreshPublicKeysFile(); err != nil {
			return fmt.Errorf("failed to load JWKS file %q: %w", p.PublicKeysFile, err)
		}

		e.providerContainers[p.IssuerURL] = &pc

		return nil
	}

	pc.transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 10 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	cert, err := getIssuerCACertFromPath(p.IssuerCAPath)
	if err != nil {
		pc.transport.CloseIdleConnections()
		return err // the errors from this path have enough context already
	}

	if cert != nil {
		pc.transport.TLSClientConfig = &tls.Config{
			RootCAs: x509.NewCertPool(),
		}
		pc.transport.TLSClientConfig.RootCAs.AddCert(cert)
	}

	pc.client = &http.Client{
		Timeout:   5 * time.Second,
		Transport: pc.transport,
	}
	oidcContext := oidc.ClientContext(ctx, pc.client)
	pc.provider, err = oidc.NewProvider(oidcContext, p.IssuerURL)
	if err != nil {
		pc.transport.CloseIdleConnections()
		pc.client.CloseIdleConnections()
		return fmt.Errorf("failed to create OIDC provider for %q: %w", p.IssuerURL, err)
	}
	pc.verifier = pc.provider.Verifier(vCfg)

	e.providerContainers[p.IssuerURL] = &pc

	return nil
}

func getSubjectFromClaims(claims map[string]any, usernameClaim, fallback string) (string, error) {
	if usernameClaim != "" {
		username, found := claims[usernameClaim]
		if !found {
			return "", errClaimNotFound
		}

		sUsername, ok := username.(string)
		if !ok {
			return "", errUsernameNotString
		}

		return sUsername, nil
	}

	return fallback, nil
}

func getGroupsFromClaims(claims map[string]any, groupsClaim string) ([]string, error) {
	if groupsClaim != "" {
		var groups []string
		rawGroup, ok := claims[groupsClaim]
		if !ok {
			return nil, errGroupsClaimNotFound
		}
		switch v := rawGroup.(type) {
		case string:
			groups = append(groups, v)
		case []string:
			groups = v
		case []any:
			groups = make([]string, 0, len(v))
			for i := range v {
				groups = append(groups, fmt.Sprintf("%v", v[i]))
			}
		}

		return groups, nil
	}

	return []string{}, nil
}

func getIssuerCACertFromPath(path string) (*x509.Certificate, error) {
	if path == "" {
		return nil, nil
	}

	rawCA, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("could not read the CA file %q: %w", path, err)
	}

	if len(rawCA) == 0 {
		return nil, fmt.Errorf("could not read the CA file %q: empty file", path)
	}

	block, _ := pem.Decode(rawCA)
	if block == nil {
		return nil, fmt.Errorf("cannot decode the contents of the CA file %q: %w", path, err)
	}

	return x509.ParseCertificate(block.Bytes)
}

func (e *oidcExtension) startFileWatchers(ctx context.Context) error {
	filesToWatch := make(map[string][]*providerContainer)

	for _, pc := range e.providerContainers {
		if pc.providerCfg.PublicKeysFile == "" {
			continue
		}

		absPath, err := filepath.Abs(pc.providerCfg.PublicKeysFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %q: %w", pc.providerCfg.PublicKeysFile, err)
		}

		if pcs, ok := filesToWatch[absPath]; ok {
			pcs = append(pcs, pc)
			filesToWatch[absPath] = pcs
		} else {
			filesToWatch[absPath] = []*providerContainer{pc}
		}
	}

	if len(filesToWatch) == 0 {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	for absPath := range filesToWatch {
		dir := filepath.Dir(absPath)
		if err := watcher.Add(dir); err != nil {
			watcher.Close()
			return fmt.Errorf("failed to watch directory %q: %w", dir, err)
		}
	}

	e.shutdownCH = make(chan struct{})
	go e.watchFiles(ctx, watcher, filesToWatch)

	return nil
}

func (e *oidcExtension) watchFiles(ctx context.Context, watcher *fsnotify.Watcher, filesToWatch map[string][]*providerContainer) {
	defer watcher.Close()

	for {
		select {
		case <-e.shutdownCH:
			return
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			absPath, err := filepath.Abs(event.Name)
			if err != nil {
				e.logger.Error("failed to get absolute path", zap.String("file", event.Name), zap.Error(err))
				continue
			}

			pcs, found := filesToWatch[absPath]
			if !found {
				continue
			}

			if event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Create == fsnotify.Create ||
				event.Op&fsnotify.Remove == fsnotify.Remove ||
				event.Op&fsnotify.Chmod == fsnotify.Chmod ||
				event.Op&fsnotify.Rename == fsnotify.Rename {
				for _, pc := range pcs {
					if err := pc.refreshPublicKeysFile(); err != nil {
						e.logger.Error("failed to reload JWKS file", zap.String("file", absPath), zap.Error(err))
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			e.logger.Error("file watcher error", zap.Error(err))
		}
	}
}

// This struct is kept minimal to avoid unnecessary allocations, especially
// if the JWT is malicious and contains too many claims.
type idToken struct {
	Issuer string `json:"iss"`
}

// Get the issuer from the raw ID token.
// This function is unsafe because it does not verify the token's signature.
// It should only be used to determine which verifier to use for the token.
func getIssuerFromUnverifiedJWT(rawIDToken string) (string, error) {
	// TODO: it would be nice if we didn't have to parse the JWT here and then again in the verifier...
	jws, err := jose.ParseSigned(rawIDToken, allSupportedAlgorithms)
	if err != nil {
		return "", fmt.Errorf("oidc: malformed jws: %w", err)
	}
	var token idToken
	if err := json.Unmarshal(jws.UnsafePayloadWithoutVerification(), &token); err != nil {
		return "", fmt.Errorf("oidc: failed to unmarshal claims: %w", err)
	}
	return token.Issuer, nil
}
