// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

type k8sKeyMaterialProvider struct {
	baseKeyMaterialProvider
}

func newK8sKeyMaterialProvider(ctx context.Context, cfg *K8sSecretConfig, logger *zap.Logger) (KeyMaterialProvider, error) {
	client, err := getK8sClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}
	return newK8sKeyMaterialProviderWithClient(ctx, client, cfg, logger)
}

func newK8sKeyMaterialProviderWithClient(ctx context.Context, client kubernetes.Interface, cfg *K8sSecretConfig, logger *zap.Logger) (KeyMaterialProvider, error) {
	// HMAC mode: load only the symmetric key
	if cfg.HMACKey != "" {
		data, err := fetchSecretDataWithClient(ctx, client, cfg.Name, cfg.Namespace, cfg.HMACKey, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch HMAC key from k8s secret: %w", err)
		}
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("HMAC key in secret %s/%s key %q: content must be standard base64-encoded: %w", cfg.Namespace, cfg.Name, cfg.HMACKey, err)
		}
		if len(key) == 0 {
			return nil, fmt.Errorf("HMAC key in secret %s/%s key %q is empty after base64 decoding", cfg.Namespace, cfg.Name, cfg.HMACKey)
		}
		return &k8sKeyMaterialProvider{baseKeyMaterialProvider{hmacKey: key}}, nil
	}

	// Asymmetric mode: load cert + private key
	certPEM, err := fetchSecretDataWithClient(ctx, client, cfg.Name, cfg.Namespace, cfg.Certificate, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch certificate from k8s secret: %w", err)
	}
	keyPEM, err := fetchSecretDataWithClient(ctx, client, cfg.Name, cfg.Namespace, cfg.PrivateKey, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch private key from k8s secret: %w", err)
	}
	certPEM = decodeIfBase64(certPEM)
	keyPEM = decodeIfBase64(keyPEM)
	certPEM = normalizeLineEndings(certPEM)
	keyPEM = normalizeLineEndings(keyPEM)
	reader, err := parseCertificateData(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &k8sKeyMaterialProvider{baseKeyMaterialProvider{reader: reader}}, nil
}
