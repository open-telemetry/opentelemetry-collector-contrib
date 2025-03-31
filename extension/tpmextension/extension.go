// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tpmextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tpmextension"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"

	keyfile "github.com/foxboron/go-tpm-keyfiles"
	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpmutil"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
)

type TPMExtension struct {
	config            *Config
	cancel            context.CancelFunc
	telemetrySettings component.TelemetrySettings

	tlsConfig *tls.Config
}

var (
	_ extension.Extension      = (*TPMExtension)(nil)
	_ extensionauth.HTTPClient = (*TPMExtension)(nil)
	//_ extensionauth.GRPCClient = (*TPMExtension)(nil)
)

var _ extension.Extension = (*TPMExtension)(nil)

func newTPMExtension(extensionCfg *Config, settings extension.Settings) (extension.Extension, error) {
	settingsExtension := &TPMExtension{
		config:            extensionCfg,
		telemetrySettings: settings.TelemetrySettings,
	}
	return settingsExtension, nil
}

func (extension *TPMExtension) Start(_ context.Context, _ component.Host) error {
	extension.telemetrySettings.Logger.Info("starting up tpm extension")
	tpm, err := tpmutil.OpenTPM(extension.config.Path)
	if err != nil {
		return err
	}
	c, err := os.ReadFile(extension.config.ClientKeyFile)
	if err != nil {
		return err
	}
	tss2Key, err := keyfile.Decode(c)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", extension.config.ClientKeyFile, err)
	}

	clientCert, err := loadCert(extension.config.ClientCertFile)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", extension.config.ClientCertFile, err)
	}
	caCert, err := loadCert(extension.config.CaFile)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", extension.config.CaFile, err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(caCert)

	signer, err := tss2Key.Signer(transport.FromReadWriteCloser(tpm), []byte(extension.config.OwnerAuth), []byte(extension.config.Auth))
	if err != nil {
		return fmt.Errorf("failed to create TPM signer: %w", err)
	}

	tlsCert := tls.Certificate{
		Certificate: [][]byte{clientCert.Raw},
		PrivateKey:  signer,
		Leaf:        clientCert,
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		RootCAs:      caCertPool,
		ServerName:   extension.config.ServerName,
	}

	extension.tlsConfig = tlsCfg
	return nil
}

func (extension *TPMExtension) Shutdown(_ context.Context) error {
	extension.telemetrySettings.Logger.Info("shutting down tmp extension")
	if extension.cancel != nil {
		extension.cancel()
	}
	return nil
}

func (extension *TPMExtension) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return &TPMRoundTripper{
		baseTransport: base,
		tpmTLSTransport: &http.Transport{
			TLSClientConfig: extension.tlsConfig,
		},
	}, nil
}

type TPMRoundTripper struct {
	baseTransport   http.RoundTripper
	tpmTLSTransport *http.Transport
}

// RoundTrip modifies the original request and adds Bearer token Authorization headers. Incoming requests support multiple tokens, but outgoing requests only use one.
func (interceptor *TPMRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return interceptor.tpmTLSTransport.RoundTrip(req)
}

func loadCert(cert string) (*x509.Certificate, error) {
	certPEM, err := os.ReadFile(cert)
	if err != nil {
		return nil, err
	}
	certDER, _ := pem.Decode(certPEM)
	if certDER == nil {
		return nil, err
	}
	leafCert, err := x509.ParseCertificate(certDER.Bytes)
	if err != nil {
		return nil, err
	}
	return leafCert, nil
}
