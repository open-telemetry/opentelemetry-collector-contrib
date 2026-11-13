// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/verifyprocessor"

import (
	"context"
	"fmt"

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
	if cfg.HMACKey != "" {
		key, err := fetchSecretDataWithClient(ctx, client, cfg.Name, cfg.Namespace, cfg.HMACKey, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch HMAC key from k8s secret: %w", err)
		}
		if len(key) == 0 {
			return nil, fmt.Errorf("HMAC key in secret %s/%s key %q is empty", cfg.Namespace, cfg.Name, cfg.HMACKey)
		}
		return &k8sKeyMaterialProvider{baseKeyMaterialProvider{hmacKey: key}}, nil
	}

	certPEM, err := fetchSecretDataWithClient(ctx, client, cfg.Name, cfg.Namespace, cfg.Certificate, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch certificate from k8s secret: %w", err)
	}
	certPEM = decodeIfBase64(certPEM)
	certPEM = normalizeLineEndings(certPEM)
	cert, err := parseCertificatePEM(certPEM)
	if err != nil {
		return nil, err
	}
	return &k8sKeyMaterialProvider{baseKeyMaterialProvider{cert: cert}}, nil
}
