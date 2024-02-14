// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" // #nosec
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"time"
)

// oidcServer is an overly simplified OIDC mock server, good enough to sign the tokens required by the test
// and pass the verification done by the underlying libraries
type oidcServer struct {
	*httptest.Server
	x509Cert   []byte
	privateKey *rsa.PrivateKey
}

func newOIDCServer() (*oidcServer, error) {
	jwks := map[string]any{}

	mux := http.NewServeMux()
	server := httptest.NewUnstartedServer(mux)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, req *http.Request) {
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
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(jwks); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	privateKey, err := createPrivateKey()
	if err != nil {
		return nil, err
	}

	x509Cert, err := createCertificate(privateKey)
	if err != nil {
		return nil, err
	}

	eBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(eBytes, uint64(privateKey.E))
	eBytes = bytes.TrimLeft(eBytes, "\x00")

	// #nosec
	sum := sha1.Sum(x509Cert)
	jwks["keys"] = []map[string]any{{
		"alg": "RS256",
		"kty": "RSA",
		"use": "sig",
		"x5c": []string{base64.StdEncoding.EncodeToString(x509Cert)},
		"n":   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(eBytes),
		"kid": base64.RawURLEncoding.EncodeToString(sum[:]),
		"x5t": base64.RawURLEncoding.EncodeToString(sum[:]),
	}}

	return &oidcServer{server, x509Cert, privateKey}, nil
}

func (s *oidcServer) token(jsonPayload []byte) (string, error) {
	jsonHeader, err := json.Marshal(map[string]any{
		"alg": "RS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}

	header := base64.RawURLEncoding.EncodeToString(jsonHeader)
	payload := base64.RawURLEncoding.EncodeToString(jsonPayload)
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s.%s", header, payload)))

	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}

	encodedSignature := base64.RawURLEncoding.EncodeToString(signature)
	token := fmt.Sprintf("%s.%s.%s", header, payload, encodedSignature)
	return token, nil
}

func createCertificate(privateKey *rsa.PrivateKey) ([]byte, error) {
	cert := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Ecorp, Inc"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(5 * time.Minute),
	}

	x509Cert, err := x509.CreateCertificate(rand.Reader, &cert, &cert, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, err
	}

	return x509Cert, nil
}

func createPrivateKey() (*rsa.PrivateKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return priv, nil
}
