// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/verifyprocessor"

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

func parseCertificatePEM(certPEM []byte) (*x509.Certificate, error) {
	if len(certPEM) == 0 {
		return nil, errors.New("certificate data is empty")
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
	return cert, nil
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
