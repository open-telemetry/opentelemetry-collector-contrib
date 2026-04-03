// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"
)

// In order to keep it simple and low risk, sharedWatchClient intentionally shares the whole watcher/cache-backed kube.Client
type sharedWatchClient struct {
	logger                 *zap.Logger
	telemetrySettings      component.TelemetrySettings
	clientProvider         kube.ClientProvider
	apiConfig              k8sconfig.APIConfig
	rules                  kube.ExtractionRules
	filters                kube.Filters
	podAssociations        []kube.Association
	podIgnore              kube.Excludes
	waitForMetadata        bool
	waitForMetadataTimeout time.Duration

	key        string
	instanceID string

	mu     sync.RWMutex
	client kube.Client
}

func newSharedWatchClient(
	logger *zap.Logger,
	telemetrySettings component.TelemetrySettings,
	clientProvider kube.ClientProvider,
	apiConfig k8sconfig.APIConfig,
	rules kube.ExtractionRules,
	filters kube.Filters,
	podAssociations []kube.Association,
	podIgnore kube.Excludes,
	waitForMetadata bool,
	waitForMetadataTimeout time.Duration,
	key string,
) *sharedWatchClient {
	return &sharedWatchClient{
		logger:                 logger,
		telemetrySettings:      telemetrySettings,
		clientProvider:         clientProvider,
		apiConfig:              apiConfig,
		rules:                  rules,
		filters:                filters,
		podAssociations:        podAssociations,
		podIgnore:              podIgnore,
		waitForMetadata:        waitForMetadata,
		waitForMetadataTimeout: waitForMetadataTimeout,
		key:                    key,
		instanceID:             uuid.NewString(),
	}
}

func buildSharedWatchClientKey(cfg *Config) (string, error) {
	// First phase intentionally hashes the full defaulted config. Any config delta, including list order,
	// produces a different key so only strict-equivalent processors share the same watcher/cache state.
	rawCfg, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal shared watch config: %w", err)
	}

	sum := sha256.Sum256(rawCfg)
	return hex.EncodeToString(sum[:]), nil
}

func (c *sharedWatchClient) currentClient() kube.Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

func (c *sharedWatchClient) getOrCreateClient() (kube.Client, error) {
	if client := c.currentClient(); client != nil {
		return client, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		return c.client, nil
	}

	clientProvider := c.clientProvider
	if clientProvider == nil {
		clientProvider = kube.New
	}

	client, err := clientProvider(
		c.telemetrySettings,
		c.apiConfig,
		c.rules,
		c.filters,
		c.podAssociations,
		c.podIgnore,
		nil,
		kube.InformersFactoryList{},
		c.waitForMetadata,
		c.waitForMetadataTimeout,
	)
	if err != nil {
		return nil, err
	}

	c.client = client
	return c.client, nil
}

func (c *sharedWatchClient) Start(context.Context, component.Host) error {
	client, err := c.getOrCreateClient()
	if err != nil {
		return err
	}

	c.logger.Info(
		"starting shared k8sattributes watch client",
		zap.String("shared_cache_instance_id", c.instanceID),
		zap.String("shared_cache_key", c.key),
	)

	return client.Start()
}

func (c *sharedWatchClient) Shutdown(context.Context) error {
	client := c.currentClient()
	if client == nil {
		return nil
	}

	c.logger.Info(
		"stopping shared k8sattributes watch client",
		zap.String("shared_cache_instance_id", c.instanceID),
		zap.String("shared_cache_key", c.key),
	)
	client.Stop()
	return nil
}

func (c *sharedWatchClient) GetPod(identifier kube.PodIdentifier) (*kube.Pod, bool) {
	client := c.currentClient()
	if client == nil {
		return nil, false
	}
	return client.GetPod(identifier)
}

func (c *sharedWatchClient) GetNamespace(namespace string) (*kube.Namespace, bool) {
	client := c.currentClient()
	if client == nil {
		return nil, false
	}
	return client.GetNamespace(namespace)
}

func (c *sharedWatchClient) GetNode(nodeName string) (*kube.Node, bool) {
	client := c.currentClient()
	if client == nil {
		return nil, false
	}
	return client.GetNode(nodeName)
}

func (c *sharedWatchClient) GetDeployment(deploymentUID string) (*kube.Deployment, bool) {
	client := c.currentClient()
	if client == nil {
		return nil, false
	}
	return client.GetDeployment(deploymentUID)
}

func (c *sharedWatchClient) GetStatefulSet(statefulsetUID string) (*kube.StatefulSet, bool) {
	client := c.currentClient()
	if client == nil {
		return nil, false
	}
	return client.GetStatefulSet(statefulsetUID)
}

func (c *sharedWatchClient) GetDaemonSet(daemonsetUID string) (*kube.DaemonSet, bool) {
	client := c.currentClient()
	if client == nil {
		return nil, false
	}
	return client.GetDaemonSet(daemonsetUID)
}

func (c *sharedWatchClient) GetJob(jobUID string) (*kube.Job, bool) {
	client := c.currentClient()
	if client == nil {
		return nil, false
	}
	return client.GetJob(jobUID)
}

func (c *sharedWatchClient) unwrapClient() kube.Client {
	return c.currentClient()
}
