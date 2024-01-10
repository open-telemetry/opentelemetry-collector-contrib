// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"

	"google.golang.org/grpc/credentials"
)

// caPool loads CA certificate from a file and returns a CertPool.
// The certPool is used to set RootCAs in certificate verification.
func caPool(caFile string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if caFile != "" {
		data, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		if !pool.AppendCertsFromPEM(data) {
			return nil, errors.New("failed to add CA certificate to root CA pool")
		}
	}
	return pool, nil
}

func GetTLSCredentialsForGRPCExporter(caFile string, cAuth ClientAuth) (credentials.TransportCredentials, error) {

	pool, err := caPool(caFile)
	if err != nil {
		return nil, err
	}

	creds := credentials.NewTLS(&tls.Config{
		RootCAs: pool,
	})

	// Configuration for mTLS
	if cAuth.Enabled {
		keypair, err := tls.LoadX509KeyPair(cAuth.ClientCertFile, cAuth.ClientKeyFile)
		if err != nil {
			return nil, err
		}
		creds = credentials.NewTLS(&tls.Config{
			RootCAs:      pool,
			Certificates: []tls.Certificate{keypair},
		})
	}

	return creds, nil
}

func GetTLSCredentialsForHTTPExporter(caFile string, cAuth ClientAuth) (*tls.Config, error) {
	pool, err := caPool(caFile)
	if err != nil {
		return nil, err
	}

	tlsCfg := tls.Config{
		RootCAs: pool,
	}

	// Configuration for mTLS
	if cAuth.Enabled {
		keypair, err := tls.LoadX509KeyPair(cAuth.ClientCertFile, cAuth.ClientKeyFile)
		if err != nil {
			return nil, err
		}
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
		tlsCfg.Certificates = []tls.Certificate{keypair}
	}
	return &tlsCfg, nil
}
