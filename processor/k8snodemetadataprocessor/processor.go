// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8snodemetadataprocessor

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	api_v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	nodeNameAttr    = "k8s.node.name"
	taintAttrPrefix = "k8s.node.taint."
	resyncPeriod    = time.Duration(0)
	nodeNameEnvVar  = "K8S_NODE_NAME"
)

// nodeTaints holds cached taint data for a single node.
// Keys in attrs are pre-computed (e.g. "k8s.node.taint.dedicated").
type nodeTaints struct {
	attrs map[string]string // attrKey -> value
}

type k8sNodeMetadataProcessor struct {
	cfg    *Config
	logger *zap.Logger

	mu           sync.RWMutex
	nodes        map[string]*nodeTaints // keyed by node name
	informer     cache.SharedInformer
	stopCh       chan struct{}
	shutdownOnce sync.Once
}

func newProcessor(cfg *Config, logger *zap.Logger) (*k8sNodeMetadataProcessor, error) {
	return &k8sNodeMetadataProcessor{
		cfg:    cfg,
		logger: logger,
		nodes:  make(map[string]*nodeTaints),
		stopCh: make(chan struct{}),
	}, nil
}

// resolveLocalNodeName determines the local node name from environment or hostname.
func resolveLocalNodeName() (string, error) {
	if name := os.Getenv(nodeNameEnvVar); name != "" {
		return name, nil
	}
	name, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("local_mode enabled but cannot determine node name: %w", err)
	}
	if name == "" {
		return "", fmt.Errorf("local_mode enabled but node name is empty (K8S_NODE_NAME unset and hostname empty)")
	}
	return name, nil
}

func (p *k8sNodeMetadataProcessor) Start(_ context.Context, _ component.Host) error {
	client, err := k8sconfig.MakeClient(p.cfg.APIConfig)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	var nodeFilter string
	if p.cfg.LocalMode {
		nodeFilter, err = resolveLocalNodeName()
		if err != nil {
			return err
		}
		p.logger.Info("local_mode enabled, watching single node", zap.String("node", nodeFilter))
	}

	p.informer = k8sconfig.NewNodeSharedInformer(client, nodeFilter, resyncPeriod)

	_, err = p.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    p.handleNodeAdd,
		UpdateFunc: p.handleNodeUpdate,
		DeleteFunc: p.handleNodeDelete,
	})
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	go p.informer.Run(p.stopCh)
	return nil
}

func (p *k8sNodeMetadataProcessor) Shutdown(_ context.Context) error {
	p.shutdownOnce.Do(func() {
		close(p.stopCh)
	})
	return nil
}

func (p *k8sNodeMetadataProcessor) handleNodeAdd(obj any) {
	node, ok := obj.(*api_v1.Node)
	if !ok {
		return
	}
	p.updateNodeTaints(node)
}

func (p *k8sNodeMetadataProcessor) handleNodeUpdate(_, newObj any) {
	node, ok := newObj.(*api_v1.Node)
	if !ok {
		return
	}
	p.updateNodeTaints(node)
}

func (p *k8sNodeMetadataProcessor) handleNodeDelete(obj any) {
	node, ok := obj.(*api_v1.Node)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		node, ok = tombstone.Obj.(*api_v1.Node)
		if !ok {
			return
		}
	}
	p.mu.Lock()
	delete(p.nodes, node.Name)
	p.mu.Unlock()
}

func (p *k8sNodeMetadataProcessor) updateNodeTaints(node *api_v1.Node) {
	attrs := make(map[string]string, len(node.Spec.Taints))
	for _, t := range node.Spec.Taints {
		attrKey := taintAttrPrefix + t.Key
		if _, exists := attrs[attrKey]; !exists {
			attrs[attrKey] = t.Value
		}
	}
	p.mu.Lock()
	p.nodes[node.Name] = &nodeTaints{attrs: attrs}
	p.mu.Unlock()
}

func (p *k8sNodeMetadataProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resourceAttrs := rm.Resource().Attributes()

		nodeNameVal, ok := resourceAttrs.Get(nodeNameAttr)
		if !ok {
			continue
		}
		nodeName := nodeNameVal.Str()
		if nodeName == "" {
			continue
		}

		p.mu.RLock()
		nt, exists := p.nodes[nodeName]
		p.mu.RUnlock()
		if !exists {
			continue
		}

		for attrKey, value := range nt.attrs {
			resourceAttrs.PutStr(attrKey, value)
		}
	}
	return md, nil
}

func (p *k8sNodeMetadataProcessor) processLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		resourceAttrs := rl.Resource().Attributes()

		nodeNameVal, ok := resourceAttrs.Get(nodeNameAttr)
		if !ok {
			continue
		}
		nodeName := nodeNameVal.Str()
		if nodeName == "" {
			continue
		}

		p.mu.RLock()
		nt, exists := p.nodes[nodeName]
		p.mu.RUnlock()
		if !exists {
			continue
		}

		for attrKey, value := range nt.attrs {
			resourceAttrs.PutStr(attrKey, value)
		}
	}
	return ld, nil
}