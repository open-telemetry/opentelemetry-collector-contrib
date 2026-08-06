// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func getK8sClient() (kubernetes.Interface, error) {
	var config *rest.Config
	var err error

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home := os.Getenv("HOME")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		if home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	if kubeconfig != "" {
		if _, err := os.Stat(kubeconfig); err == nil {
			config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
			}
		}
	}

	if config == nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config (trying to use service account token): %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return clientset, nil
}

func fetchSecretData(ctx context.Context, secretName, namespace, key string, logger *zap.Logger) ([]byte, error) {
	client, err := getK8sClient()
	if err != nil {
		if logger != nil {
			logger.Error("Failed to create k8s client", zap.Error(err))
		}
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}
	return fetchSecretDataWithClient(ctx, client, secretName, namespace, key, logger)
}

func fetchSecretDataWithClient(ctx context.Context, client kubernetes.Interface, secretName, namespace, key string, logger *zap.Logger) ([]byte, error) {
	if logger != nil {
		logger.Info("Fetching secret data",
			zap.String("secret", fmt.Sprintf("%s/%s", namespace, secretName)),
			zap.String("key", key),
		)
	}

	maxRetries := 30
	retryDelay := 2 * time.Second
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		secret, err := client.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				if attempt < maxRetries-1 {
					if logger != nil {
						logger.Info("Secret not found, retrying...",
							zap.String("secret", fmt.Sprintf("%s/%s", namespace, secretName)),
							zap.Int("attempt", attempt+1),
							zap.Int("max_attempts", maxRetries),
							zap.Duration("retry_delay", retryDelay),
						)
					}
					select {
					case <-ctx.Done():
						return nil, fmt.Errorf("context cancelled while waiting to retry secret %s/%s: %w", namespace, secretName, ctx.Err())
					case <-time.After(retryDelay):
					}
					retryDelay = time.Duration(float64(retryDelay) * 1.5)
					if retryDelay > 10*time.Second {
						retryDelay = 10 * time.Second
					}
					continue
				}
				lastErr = fmt.Errorf("secret %s/%s not found after %d attempts", namespace, secretName, maxRetries)
			} else {
				if logger != nil {
					logger.Error("Failed to get secret",
						zap.Error(err),
						zap.String("secret", fmt.Sprintf("%s/%s", namespace, secretName)),
						zap.Int("attempt", attempt+1),
					)
				}
				return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, secretName, err)
			}
		} else {
			if logger != nil && attempt > 0 {
				logger.Info("Secret found successfully",
					zap.String("secret", fmt.Sprintf("%s/%s", namespace, secretName)),
					zap.Int("attempts", attempt+1),
				)
			}
			data, exists := secret.Data[key]
			if !exists {
				availableKeys := make([]string, 0, len(secret.Data))
				for k := range secret.Data {
					availableKeys = append(availableKeys, k)
				}
				err := fmt.Errorf("key %s not found in secret %s/%s (available keys: %v)", key, namespace, secretName, availableKeys)
				if logger != nil {
					logger.Error("Key not found in secret",
						zap.String("key", key),
						zap.String("secret", fmt.Sprintf("%s/%s", namespace, secretName)),
						zap.Strings("available_keys", availableKeys),
					)
				}
				return nil, err
			}
			if logger != nil {
				logger.Info("Successfully fetched secret data",
					zap.String("secret", fmt.Sprintf("%s/%s", namespace, secretName)),
					zap.String("key", key),
					zap.Int("data_length", len(data)),
				)
			}
			return data, nil
		}
	}

	return nil, lastErr
}
