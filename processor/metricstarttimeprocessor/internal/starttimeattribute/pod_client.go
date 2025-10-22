package starttimeattribute

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type k8sPodClient struct {
	useContainerReadiness bool
	clientset             k8s.Interface
	informerStop          chan struct{}
	informer              cache.SharedIndexInformer
	mu                    sync.RWMutex
	startTimeCache        map[string]time.Time
}

const defaultCacheSyncDuration = 10 * time.Minute

func newK8sPodClient(_ context.Context, apiConfig k8sconfig.APIConfig, filter informerFilter, useContainerReadiness bool) (podClient, error) {
	clientset, err := k8sconfig.MakeClient(apiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	labelSelector := labels.Everything()
	if len(filter.labelFilters) > 0 {
		for _, lf := range filter.labelFilters {
			requirement, err := labels.NewRequirement(lf.Key, lf.Op, []string{lf.Value})
			if err != nil {
				return nil, fmt.Errorf("failed to create label requirement: %w", err)
			}
			labelSelector = labelSelector.Add(*requirement)
		}
	}

	fieldSelectors := []fields.Selector{fields.Everything()}
	if filter.node != "" {
		fieldSelectors = append(fieldSelectors, fields.OneTermEqualSelector("spec.nodeName", filter.node))
	}
	fieldSelector := fields.AndSelectors(fieldSelectors...)
	options := []informers.SharedInformerOption{informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
		opts.LabelSelector = labelSelector.String()
		opts.FieldSelector = fieldSelector.String()
	})}
	if filter.namespace != "" {
		options = append(options, informers.WithNamespace(filter.namespace))
	}

	factory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		defaultCacheSyncDuration,
		options...,
	)

	podInformer := factory.Core().V1().Pods().Informer()
	client := &k8sPodClient{
		clientset:             clientset,
		informerStop:          make(chan struct{}),
		informer:              podInformer,
		startTimeCache:        make(map[string]time.Time),
		useContainerReadiness: useContainerReadiness,
	}

	_, err = podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return
			}
			client.mu.Lock()
			defer client.mu.Unlock()
			client.addPod(pod)

		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod, ok := oldObj.(*corev1.Pod)
			if !ok {
				return
			}
			newPod, ok := newObj.(*corev1.Pod)
			if !ok {
				return
			}
			client.mu.Lock()
			defer client.mu.Unlock()

			client.deletePod(oldPod)
			client.addPod(newPod)
		},
		DeleteFunc: func(obj interface{}) {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return
			}
			client.mu.Lock()
			defer client.mu.Unlock()
			client.deletePod(pod)
		},
	})
	if err != nil {
		return nil, err
	}
	factory.Start(client.informerStop)
	factory.WaitForCacheSync(client.informerStop)

	return client, nil
}

func (c *k8sPodClient) addPod(pod *corev1.Pod) {
	podName := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	podStatus := pod.Status
	var startTime time.Time
	if c.useContainerReadiness {
		ready, readyTime := c.containerReadinessTime(pod)
		if ready {
			startTime = readyTime
		}
	} else {
		if podStatus.StartTime != nil {
			startTime = podStatus.StartTime.Time
		}
	}
	if startTime.IsZero() {
		return
	}
	if podStatus.PodIP != "" {
		c.startTimeCache[fmt.Sprintf("ip:%s", podStatus.PodIP)] = startTime
	}
	c.startTimeCache[fmt.Sprintf("name:%s", podName)] = startTime
	c.startTimeCache[fmt.Sprintf("uid:%s", pod.UID)] = startTime
}

func (c *k8sPodClient) containerReadinessTime(pod *corev1.Pod) (bool, time.Time) {
	var containerReadyTime time.Time
	var podReady, containersReady bool
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			if condition.Status == corev1.ConditionTrue {
				podReady = true
			} else {
				return false, containerReadyTime
			}
		}
		if condition.Type == corev1.ContainersReady {
			if condition.Status == corev1.ConditionTrue {
				containersReady = true
				containerReadyTime = condition.LastTransitionTime.Time
			} else {
				return false, containerReadyTime
			}
		}
	}
	return podReady && containersReady, containerReadyTime
}

func (c *k8sPodClient) deletePod(pod *corev1.Pod) {
	if pod.Status.PodIP != "" {
		delete(c.startTimeCache, fmt.Sprintf("ip:%s", pod.Status.PodIP))
	}

	podName := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	delete(c.startTimeCache, fmt.Sprintf("name:%s", podName))
	delete(c.startTimeCache, fmt.Sprintf("uid:%s", pod.UID))
}

func (c *k8sPodClient) GetPodStartTime(_ context.Context, podID podIdentifier) time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var cacheKey string

	switch podID.Type {
	case podIP:
		cacheKey = fmt.Sprintf("ip:%s", podID.Value)
	case podName:
		cacheKey = fmt.Sprintf("name:%s", podID.Value)
	case podUID:
		cacheKey = fmt.Sprintf("uid:%s", podID.Value)
	default:
		return time.Time{}
	}

	if startTime, ok := c.startTimeCache[cacheKey]; ok {
		return startTime
	}

	return time.Time{}
}

func (c *k8sPodClient) Stop() {
	close(c.informerStop)
}
