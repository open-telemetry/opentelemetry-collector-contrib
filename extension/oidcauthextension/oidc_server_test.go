// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" // #nosec
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/require"
)

// oidcServer is an overly simplified OIDC mock server, good enough to sign the tokens required by the test
// and pass the verification done by the underlying libraries
type oidcServer struct {
	*httptest.Server
	x509Cert   []byte
	privateKey crypto.Signer
	alg        jose.SignatureAlgorithm
}

func newOIDCServer() (*oidcServer, error) {
	return newOIDCServerWithAlg(jose.RS256)
}

func newOIDCServerWithAlg(alg jose.SignatureAlgorithm) (*oidcServer, error) {
	jwks := jose.JSONWebKeySet{}

	mux := http.NewServeMux()
	server := httptest.NewUnstartedServer(mux)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		err := json.NewEncoder(w).Encode(map[string]any{
			"issuer":   server.URL,
			"jwks_uri": fmt.Sprintf("%s/.well-known/jwks.json", server.URL),
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(jwks); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	privateKey, err := createPrivateKey(alg)
	if err != nil {
		return nil, err
	}

	x509Cert, err := createCertificate(privateKey)
	if err != nil {
		return nil, err
	}

	// #nosec
	sum := sha1.Sum(x509Cert)
	kid := base64.RawURLEncoding.EncodeToString(sum[:])

	jwk := jose.JSONWebKey{
		Key:       privateKey.Public(),
		KeyID:     kid,
		Algorithm: string(alg),
		Use:       "sig",
	}

	jwks.Keys = []jose.JSONWebKey{jwk}

	return &oidcServer{server, x509Cert, privateKey, alg}, nil
}

func newReverseProxy(t *testing.T, dst string) *httptest.Server {
	remote, err := url.Parse(dst)
	require.NoError(t, err)

	handler := func(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			r.Host = remote.Host
			p.ServeHTTP(w, r)
		}
	}
	proxy := httputil.NewSingleHostReverseProxy(remote)

	server := httptest.NewServer(http.HandlerFunc(handler(proxy)))
	return server
}

func (s *oidcServer) token(jsonPayload []byte) (string, error) {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: s.alg, Key: s.privateKey}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		return "", err
	}

	object, err := signer.Sign(jsonPayload)
	if err != nil {
		return "", err
	}

	return object.CompactSerialize()
}

func createCertificate(privateKey crypto.Signer) ([]byte, error) {
	cert := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Ecorp, Inc"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(5 * time.Minute),
	}

	x509Cert, err := x509.CreateCertificate(rand.Reader, &cert, &cert, privateKey.Public(), privateKey)
	if err != nil {
		return nil, err
	}

	return x509Cert, nil
}

func createPrivateKey(alg jose.SignatureAlgorithm) (crypto.Signer, error) {
	switch alg {
	case jose.RS256:
		return rsa.GenerateKey(rand.Reader, 2048)
	case jose.ES256:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", alg)
	}
}
