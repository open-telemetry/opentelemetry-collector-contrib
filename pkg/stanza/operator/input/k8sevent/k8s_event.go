// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sevent // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/k8sevent"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func init() {
	operator.Register("k8s_event_input", func() operator.Builder { return NewK8sEventsConfig("") })
}

// NewK8sEventsConfig creates a default K8sEventsConfig
func NewK8sEventsConfig(operatorID string) *K8sEventsConfig {
	return &K8sEventsConfig{
		InputConfig:        helper.NewInputConfig(operatorID, "k8s_event_input"),
		Namespaces:         []string{},
		DiscoverNamespaces: true,
		DiscoveryInterval:  helper.Duration{Duration: time.Minute * 1},
	}
}

// K8sEventsConfig is the configuration of K8sEvents operator
type K8sEventsConfig struct {
	helper.InputConfig `yaml:",inline"`
	Namespaces         []string        `json:"namespaces" yaml:"namespaces"`
	DiscoverNamespaces bool            `json:"discover_namespaces" yaml:"discover_namespaces"`
	DiscoveryInterval  helper.Duration `json:"discovery_interval" yaml:"discovery_interval"`
}

// Build will build a k8s_event_input operator from the supplied configuration
func (c K8sEventsConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	input, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, errors.Wrap(err, "build transformer")
	}

	if len(c.Namespaces) == 0 && !c.DiscoverNamespaces {
		return nil, fmt.Errorf("`namespaces` must be specified or `discover_namespaces` enabled")
	}

	return &K8sEvents{
		InputOperator:      input,
		namespaces:         c.Namespaces,
		discoverNamespaces: c.DiscoverNamespaces,
		discoveryInterval:  c.DiscoveryInterval,
	}, nil
}

// K8sEvents is an operator for generating logs from k8s events
type K8sEvents struct {
	helper.InputOperator
	client             corev1.CoreV1Interface
	discoverNamespaces bool
	discoveryInterval  helper.Duration
	namespaces         []string

	cancel       func()
	wg           sync.WaitGroup
	namespaceMux sync.Mutex
}

// Start implements the operator.Operator interface
func (k *K8sEvents) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	k.cancel = cancel

	// Currently, we only support running in the cluster. In contrast to the
	// k8s_metadata_decorator, it may make sense to relax this restriction
	// by exposing client config options.
	config, err := rest.InClusterConfig()
	if err != nil {
		return errors.NewError(
			"agent not in kubernetes cluster",
			"the k8s_event_input operator only supports running in a pod inside a kubernetes cluster",
		)
	}

	k.client, err = corev1.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "build client")
	}

	if k.discoverNamespaces {
		namespaces, err := listNamespaces(ctx, k.client)
		if err != nil {
			return errors.Wrap(err, "initial namespace discovery")
		}

		for _, namespace := range namespaces {
			if !k.hasNamespace(namespace) {
				k.addNamespace(namespace)
			}
		}
	}

	// Test connection
	if len(k.namespaces) > 0 {
		testWatcher, err := k.client.Events(k.namespaces[0]).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("test connection failed: list events for namespace '%s': %s", k.namespaces[0], err)
		}
		testWatcher.Stop()
	}

	for _, ns := range k.namespaces {
		k.startWatchingNamespace(ctx, ns)
	}

	// Find and watch namespaces if in discovery mode
	if k.discoverNamespaces {
		k.startFindingNamespaces(ctx, k.client)
	}

	return nil
}

// Stop implements operator.Operator
func (k *K8sEvents) Stop() error {
	k.cancel()
	k.wg.Wait()
	return nil
}

// listNamespaces gets a full list of namespaces from the client
func listNamespaces(ctx context.Context, client corev1.CoreV1Interface) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := client.Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	namespaces := make([]string, 0, 10)
	for _, ns := range res.Items {
		namespaces = append(namespaces, ns.Name)
	}
	return namespaces, nil
}

// startFindingNamespaces creates a goroutine that looks for namespaces to watch
func (k *K8sEvents) startFindingNamespaces(ctx context.Context, client corev1.CoreV1Interface) {
	k.wg.Add(1)
	go func() {
		defer k.wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(k.discoveryInterval.Duration):
				namespaces, err := listNamespaces(ctx, client)
				if err != nil {
					k.Errorf("failed to list namespaces: %s", err)
					continue
				}

				for _, namespace := range namespaces {
					if k.hasNamespace(namespace) {
						continue
					}

					k.addNamespace(namespace)
					k.startWatchingNamespace(ctx, namespace)
				}
			}
		}
	}()
}

// startWatchingNamespace creates a goroutine that watches the events for a
// specific namespace
func (k *K8sEvents) startWatchingNamespace(ctx context.Context, ns string) {
	k.wg.Add(1)
	go func() {
		defer k.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			watcher, err := k.client.Events(ns).Watch(ctx, metav1.ListOptions{})
			if err != nil {
				k.Errorw("Failed to start watcher", zap.Error(err))
				k.removeNamespace(ns)
				return
			}

			k.consumeWatchEvents(ctx, watcher.ResultChan())
		}
	}()
}

// addNamespace will add a namespace.
func (k *K8sEvents) addNamespace(namespace string) {
	k.namespaceMux.Lock()
	k.namespaces = append(k.namespaces, namespace)
	k.namespaceMux.Unlock()
}

// hasNamespace returns a boolean indicating if the namespace is being watched.
func (k *K8sEvents) hasNamespace(namespace string) bool {
	k.namespaceMux.Lock()
	defer k.namespaceMux.Unlock()

	for _, n := range k.namespaces {
		if n == namespace {
			return true
		}
	}
	return false
}

// removeNamespace removes a namespace.
func (k *K8sEvents) removeNamespace(namespace string) {
	k.namespaceMux.Lock()
	defer k.namespaceMux.Unlock()

	for i, n := range k.namespaces {
		if n != namespace {
			continue
		}
		k.namespaces = append(k.namespaces[:i], k.namespaces[i+1:]...)
		break
	}
}

// consumeWatchEvents will read events from the watcher channel until the channel is closed
// or the context is canceled
func (k *K8sEvents) consumeWatchEvents(ctx context.Context, events <-chan watch.Event) {
	for {
		select {
		case event, ok := <-events:
			if !ok {
				k.Error("Watcher channel closed")
				return
			}

			typedEvent := event.Object.(*apiv1.Event)
			body, err := runtime.DefaultUnstructuredConverter.ToUnstructured(event.Object)
			if err != nil {
				k.Error("Failed to convert event to map", zap.Error(err))
				continue
			}

			entry, err := k.NewEntry(body)
			if err != nil {
				k.Error("Failed to create new entry from body", zap.Error(err))
				continue
			}

			// Prioritize EventTime > LastTimestamp > FirstTimestamp
			switch {
			case typedEvent.EventTime.Time != time.Time{}:
				entry.Timestamp = typedEvent.EventTime.Time
			case typedEvent.LastTimestamp.Time != time.Time{}:
				entry.Timestamp = typedEvent.LastTimestamp.Time
			case typedEvent.FirstTimestamp.Time != time.Time{}:
				entry.Timestamp = typedEvent.FirstTimestamp.Time
			}

			entry.AddAttribute("event_type", string(event.Type))
			k.populateResource(typedEvent, entry)
			k.Write(ctx, entry)
		case <-ctx.Done():
			return
		}
	}
}

// populateResource uses the keys from Event.ObjectMeta to populate the resource of the entry
func (k *K8sEvents) populateResource(event *apiv1.Event, entry *entry.Entry) {
	io := event.InvolvedObject

	entry.AddResourceKey("k8s.cluster.name", event.ClusterName)
	entry.AddResourceKey("k8s.namespace.name", io.Namespace)

	switch io.Kind {
	case "Pod":
		entry.AddResourceKey("k8s.pod.uid", string(io.UID))
		entry.AddResourceKey("k8s.pod.name", io.Name)
	case "Container":
		entry.AddResourceKey("k8s.container.name", io.Name)
	case "ReplicaSet":
		entry.AddResourceKey("k8s.replicaset.uid", string(io.UID))
		entry.AddResourceKey("k8s.replicaset.name", io.Name)
	case "Deployment":
		entry.AddResourceKey("k8s.deployment.uid", string(io.UID))
		entry.AddResourceKey("k8s.deployment.name", io.Name)
	case "StatefulSet":
		entry.AddResourceKey("k8s.statefulset.uid", string(io.UID))
		entry.AddResourceKey("k8s.statefulset.name", io.Name)
	case "DaemonSet":
		entry.AddResourceKey("k8s.daemonset.uid", string(io.UID))
		entry.AddResourceKey("k8s.daemonset.name", io.Name)
	case "Job":
		entry.AddResourceKey("k8s.job.uid", string(io.UID))
		entry.AddResourceKey("k8s.job.name", io.Name)
	case "CronJob":
		entry.AddResourceKey("k8s.cronjob.uid", string(io.UID))
		entry.AddResourceKey("k8s.cronjob.name", io.Name)
	}
}
