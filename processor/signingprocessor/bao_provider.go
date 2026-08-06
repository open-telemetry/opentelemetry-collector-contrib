// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"context"
	"fmt"

	openbao "github.com/openbao/openbao/api/v2"
)

type baoKeyMaterialProvider struct {
	baseKeyMaterialProvider
}

// newBaoKeyMaterialProvider reads key material from an OpenBao (or Vault-compatible)
// KV secret. For asymmetric algorithms the secret must contain the fields named by
// cfg.CertField and cfg.KeyField (PEM-encoded strings). For HMAC-SHA256 it must
// contain the field named by cfg.HMACKeyField (raw or base64-encoded bytes).
func newBaoKeyMaterialProvider(ctx context.Context, cfg *BaoKeyConfig) (KeyMaterialProvider, error) {
	return newBaoKeyMaterialProviderWithAddress(ctx, cfg, "")
}

// newBaoKeyMaterialProviderWithAddress is like newBaoKeyMaterialProvider but accepts
// an explicit server address override (used in tests).
func newBaoKeyMaterialProviderWithAddress(ctx context.Context, cfg *BaoKeyConfig, addressOverride string) (KeyMaterialProvider, error) {
	clientCfg := openbao.DefaultConfig()
	switch {
	case addressOverride != "":
		clientCfg.Address = addressOverride
	case cfg.Address != "":
		clientCfg.Address = cfg.Address
	}

	client, err := openbao.NewClient(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create openbao client: %w", err)
	}
	if cfg.Token != "" {
		client.SetToken(cfg.Token)
	}

	secret, err := client.Logical().ReadWithContext(ctx, cfg.SecretPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret at %q: %w", cfg.SecretPath, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("secret at %q is empty or does not exist", cfg.SecretPath)
	}

	// HMAC mode: load only the symmetric key field
	if cfg.HMACKeyField != "" {
		raw, err := secretField(secret.Data, cfg.HMACKeyField)
		if err != nil {
			return nil, fmt.Errorf("HMAC key field %q in secret %q: %w", cfg.HMACKeyField, cfg.SecretPath, err)
		}
		key := decodeIfBase64(normalizeLineEndings([]byte(raw)))
		if len(key) == 0 {
			return nil, fmt.Errorf("HMAC key field %q in secret %q is empty after decoding", cfg.HMACKeyField, cfg.SecretPath)
		}
		return &baoKeyMaterialProvider{baseKeyMaterialProvider{hmacKey: key}}, nil
	}

	// Asymmetric mode: load cert + private key fields
	certPEM, err := secretField(secret.Data, cfg.CertField)
	if err != nil {
		return nil, fmt.Errorf("certificate field %q in secret %q: %w", cfg.CertField, cfg.SecretPath, err)
	}
	keyPEM, err := secretField(secret.Data, cfg.KeyField)
	if err != nil {
		return nil, fmt.Errorf("key field %q in secret %q: %w", cfg.KeyField, cfg.SecretPath, err)
	}

	certBytes := decodeIfBase64(normalizeLineEndings([]byte(certPEM)))
	keyBytes := decodeIfBase64(normalizeLineEndings([]byte(keyPEM)))

	reader, err := parseCertificateData(certBytes, keyBytes)
	if err != nil {
		return nil, err
	}
	return &baoKeyMaterialProvider{baseKeyMaterialProvider{reader: reader}}, nil
}

func secretField(data map[string]interface{}, field string) (string, error) {
	raw, ok := data[field]
	if !ok {
		return "", fmt.Errorf("field not found in secret data")
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("field value is not a string")
	}
	if s == "" {
		return "", fmt.Errorf("field value is empty")
	}
	return s, nil
}
