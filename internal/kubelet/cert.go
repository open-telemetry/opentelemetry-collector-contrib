// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"

import (
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"runtime"
)

func systemCertPoolPlusPath(certPath string) (*x509.CertPool, error) {
	var sysCerts *x509.CertPool
	var err error
	if runtime.GOOS == "windows" {
		sysCerts, err = x509.NewCertPool(), nil
	} else {
		sysCerts, err = x509.SystemCertPool()
	}
	if err != nil {
		return nil, fmt.Errorf("could not load system x509 cert pool: %w", err)
	}
	return certPoolPlusPath(sysCerts, certPath)
}

func certPoolPlusPath(certPool *x509.CertPool, certPath string) (*x509.CertPool, error) {
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("cert path %s could not be read: %w", certPath, err)
	}
	ok := certPool.AppendCertsFromPEM(certBytes)
	if !ok {
		return nil, errors.New("AppendCertsFromPEM failed")
	}
	return certPool, nil
}
