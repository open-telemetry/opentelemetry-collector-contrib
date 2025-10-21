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
	clientset      k8s.Interface
	informerStop   chan struct{}
	informer       cache.SharedIndexInformer
	mu             sync.RWMutex
	startTimeCache map[string]time.Time
}

func newK8sPodClient(ctx context.Context, apiConfig k8sconfig.APIConfig, filter informerFilter) (podClient, error) {
	clientset, err := k8sconfig.MakeClient(apiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	labelSelector := labels.Everything()
	if len(filter.LabelFilters) > 0 {
		for _, lf := range filter.LabelFilters {
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
	options := informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
		opts.LabelSelector = labelSelector.String()
		opts.FieldSelector = fieldSelector.String()
	})

	var factory informers.SharedInformerFactory
	if filter.namespace != "" {
		factory = informers.NewSharedInformerFactoryWithOptions(
			clientset,
			time.Hour,
			informers.WithNamespace(filter.namespace),
			options,
		)
	} else {
		factory = informers.NewSharedInformerFactoryWithOptions(
			clientset,
			time.Hour,
			options,
		)
	}

	podInformer := factory.Core().V1().Pods().Informer()

	client := &k8sPodClient{
		clientset:      clientset,
		informerStop:   make(chan struct{}),
		informer:       podInformer,
		startTimeCache: make(map[string]time.Time),
	}

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if pod, ok := obj.(*corev1.Pod); ok {
				client.addPod(pod)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if oldPod, ok := oldObj.(*corev1.Pod); ok {
				client.deletePod(oldPod)
			}
			if newPod, ok := newObj.(*corev1.Pod); ok {
				client.addPod(newPod)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if pod, ok := obj.(*corev1.Pod); ok {
				client.deletePod(pod)
			}
		},
	})

	factory.Start(client.informerStop)
	factory.WaitForCacheSync(client.informerStop)

	return client, nil
}

func (c *k8sPodClient) addPod(pod *corev1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()

	podName := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	if pod.Status.StartTime != nil {
		startTime := pod.Status.StartTime.Time
		if pod.Status.PodIP != "" {
			c.startTimeCache[fmt.Sprintf("ip:%s", pod.Status.PodIP)] = startTime
		}
		c.startTimeCache[fmt.Sprintf("name:%s", podName)] = startTime
		c.startTimeCache[fmt.Sprintf("uid:%s", pod.UID)] = startTime
	}
}

func (c *k8sPodClient) deletePod(pod *corev1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()

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
