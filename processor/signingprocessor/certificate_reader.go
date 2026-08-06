// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
)

// certificateReader holds a parsed private signing key and X.509 certificate.
// It is used by KeyMaterialProvider implementations after they have fetched
// the raw PEM data from their respective source.
type certificateReader struct {
	cert *x509.Certificate
	key  crypto.Signer
}

func (cr *certificateReader) GetPrivateKey() crypto.Signer {
	return cr.key
}

func (cr *certificateReader) GetCertificate() *x509.Certificate {
	return cr.cert
}

// GetHMACKey returns nil — certificateReader holds asymmetric key material only.
func (cr *certificateReader) GetHMACKey() []byte { return nil }

// parseCertificateData parses PEM-encoded certificate and private key bytes
// into a certificateReader. Supported key types: RSA (PKCS1 and PKCS8),
// ECDSA (SEC1 and PKCS8), and Ed25519 (PKCS8).
func parseCertificateData(certPEM, keyPEM []byte) (*certificateReader, error) {
	if len(certPEM) == 0 {
		return nil, fmt.Errorf("certificate data is empty")
	}
	if len(keyPEM) == 0 {
		return nil, fmt.Errorf("private key data is empty")
	}

	certStr := string(certPEM)
	if !strings.Contains(certStr, "-----BEGIN") {
		return nil, fmt.Errorf("certificate data does not appear to be PEM format (data length: %d, first 100 bytes: %q)", len(certPEM), string(certPEM[:min(100, len(certPEM))]))
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("failed to decode PEM certificate (data length: %d, first 100 bytes: %q)", len(certPEM), string(certPEM[:min(100, len(certPEM))]))
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	keyStr := string(keyPEM)
	if !strings.Contains(keyStr, "-----BEGIN") {
		return nil, fmt.Errorf("private key data does not appear to be PEM format (data length: %d, first 100 bytes: %q)", len(keyPEM), string(keyPEM[:min(100, len(keyPEM))]))
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode PEM private key (data length: %d, first 100 bytes: %q)", len(keyPEM), string(keyPEM[:min(100, len(keyPEM))]))
	}

	var key crypto.Signer
	switch keyBlock.Type {
	case "RSA PRIVATE KEY":
		k, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS1 RSA private key: %w", err)
		}
		key = k
	case "EC PRIVATE KEY":
		k, err := x509.ParseECPrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SEC1 EC private key: %w", err)
		}
		key = k
	case "PRIVATE KEY":
		parsed, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
		}
		signer, ok := parsed.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("unsupported PKCS8 key type: %T (expected RSA, ECDSA, or Ed25519)", parsed)
		}
		key = signer
	default:
		return nil, fmt.Errorf("unsupported private key PEM type: %q (expected RSA PRIVATE KEY, EC PRIVATE KEY, or PRIVATE KEY)", keyBlock.Type)
	}

	// Verify key type matches certificate's public key algorithm.
	switch key.(type) {
	case *rsa.PrivateKey:
		if cert.PublicKeyAlgorithm != x509.RSA {
			return nil, fmt.Errorf("RSA private key does not match certificate public key algorithm %s", cert.PublicKeyAlgorithm)
		}
	case *ecdsa.PrivateKey:
		if cert.PublicKeyAlgorithm != x509.ECDSA {
			return nil, fmt.Errorf("EC private key does not match certificate public key algorithm %s", cert.PublicKeyAlgorithm)
		}
	case ed25519.PrivateKey:
		if cert.PublicKeyAlgorithm != x509.Ed25519 {
			return nil, fmt.Errorf("Ed25519 private key does not match certificate public key algorithm %s", cert.PublicKeyAlgorithm)
		}
	}

	return &certificateReader{cert: cert, key: key}, nil
}

func decodeIfBase64(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	dataStr := strings.TrimSpace(string(data))
	if !strings.HasPrefix(dataStr, "-----BEGIN") {
		decoded, err := base64.StdEncoding.DecodeString(dataStr)
		if err == nil && len(decoded) > 0 {
			if strings.HasPrefix(string(decoded), "-----BEGIN") {
				return decoded
			}
		}
	}
	return data
}

func normalizeLineEndings(data []byte) []byte {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return []byte(s)
}
