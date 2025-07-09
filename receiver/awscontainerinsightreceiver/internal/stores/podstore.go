// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores/kubeletutil"
)

const (
	refreshInterval    = 30 * time.Second
	measurementsExpiry = 10 * time.Minute
	podsExpiry         = 2 * time.Minute
	memoryKey          = "memory"
	cpuKey             = "cpu"
	splitRegexStr      = "\\.|-"
	kubeProxy          = "kube-proxy"
)

var re = regexp.MustCompile(splitRegexStr)

type cachedEntry struct {
	pod      corev1.Pod
	creation time.Time
}

type Owner struct {
	OwnerKind string `json:"owner_kind"`
	OwnerName string `json:"owner_name"`
}

type prevPodMeasurement struct {
	containersRestarts int
}

type prevContainerMeasurement struct {
	restarts int
}

type mapWithExpiry struct {
	*awsmetrics.MapWithExpiry
}

func (m *mapWithExpiry) Get(key string) (any, bool) {
	m.Lock()
	defer m.Unlock()
	if val, ok := m.MapWithExpiry.Get(awsmetrics.NewKey(key, nil)); ok {
		return val.RawValue, ok
	}

	return nil, false
}

func (m *mapWithExpiry) Set(key string, content any) {
	m.Lock()
	defer m.Unlock()
	val := awsmetrics.MetricValue{
		RawValue:  content,
		Timestamp: time.Now(),
	}
	m.MapWithExpiry.Set(awsmetrics.NewKey(key, nil), val)
}

func newMapWithExpiry(ttl time.Duration) *mapWithExpiry {
	return &mapWithExpiry{
		MapWithExpiry: awsmetrics.NewMapWithExpiry(ttl),
	}
}

type replicaSetInfoProvider interface {
	GetReplicaSetClient() k8sclient.ReplicaSetClient
}

type podClient interface {
	ListPods() ([]corev1.Pod, error)
}

type PodStore struct {
	cache            *mapWithExpiry
	prevMeasurements map[string]*mapWithExpiry // preMeasurements per each Type (Pod, Container, etc)
	podClient        podClient
	k8sClient        replicaSetInfoProvider
	lastRefreshed    time.Time
	nodeInfo         *nodeInfo
	prefFullPodName  bool
	logger           *zap.Logger
	sync.Mutex
	addFullPodNameMetricLabel bool
}

func NewPodStore(hostIP string, prefFullPodName bool, addFullPodNameMetricLabel bool, logger *zap.Logger) (*PodStore, error) {
	podClient, err := kubeletutil.NewKubeletClient(hostIP, ci.KubeSecurePort, logger)
	if err != nil {
		return nil, err
	}

	// Try to detect kubelet permission issue here
	if _, err := podClient.ListPods(); err != nil {
		return nil, fmt.Errorf("cannot get pod from kubelet, err: %w", err)
	}

	k8sClient := k8sclient.Get(logger)
	if k8sClient == nil {
		return nil, errors.New("failed to start pod store because k8sclient is nil")
	}

	podStore := &PodStore{
		cache:                     newMapWithExpiry(podsExpiry),
		prevMeasurements:          make(map[string]*mapWithExpiry),
		podClient:                 podClient,
		nodeInfo:                  newNodeInfo(logger),
		prefFullPodName:           prefFullPodName,
		k8sClient:                 k8sClient,
		logger:                    logger,
		addFullPodNameMetricLabel: addFullPodNameMetricLabel,
	}

	return podStore, nil
}

func (p *PodStore) Shutdown() error {
	var errs error
	errs = p.cache.Shutdown()
	for _, maps := range p.prevMeasurements {
		if prevMeasErr := maps.Shutdown(); prevMeasErr != nil {
			errs = errors.Join(errs, prevMeasErr)
		}
	}
	return errs
}

func (p *PodStore) getPrevMeasurement(metricType, metricKey string) (any, bool) {
	prevMeasurement, ok := p.prevMeasurements[metricType]
	if !ok {
		return nil, false
	}

	content, ok := prevMeasurement.Get(metricKey)

	if !ok {
		return nil, false
	}

	return content, true
}

func (p *PodStore) setPrevMeasurement(metricType, metricKey string, content any) {
	prevMeasurement, ok := p.prevMeasurements[metricType]
	if !ok {
		prevMeasurement = newMapWithExpiry(measurementsExpiry)
		p.prevMeasurements[metricType] = prevMeasurement
	}
	prevMeasurement.Set(metricKey, content)
}

// RefreshTick triggers refreshing of the pod store.
// It will be called at relatively short intervals (e.g. 1 second).
// We can't do refresh in regular interval because the Decorate(...) function will
// call refresh(...) on demand when the pod metadata for the given metrics is not in
// cache yet. This will make the refresh interval irregular.
func (p *PodStore) RefreshTick(ctx context.Context) {
	now := time.Now()
	if now.Sub(p.lastRefreshed) >= refreshInterval {
		p.refresh(ctx, now)
		p.lastRefreshed = now
	}
}

func (p *PodStore) Decorate(ctx context.Context, metric CIMetric, kubernetesBlob map[string]any) bool {
	if metric.GetTag(ci.MetricType) == ci.TypeNode {
		p.decorateNode(metric)
	} else if metric.GetTag(ci.K8sPodNameKey) != "" {
		podKey := createPodKeyFromMetric(metric)
		if podKey == "" {
			p.logger.Error("podKey is unavailable when decorating pod")
			return false
		}

		entry := p.getCachedEntry(podKey)
		if entry == nil {
			p.logger.Debug(fmt.Sprintf("no pod is found for %s, refresh the cache now...", podKey))
			p.refresh(ctx, time.Now())
			entry = p.getCachedEntry(podKey)
		}

		// If pod is still not found, insert a placeholder to avoid too many refresh
		if entry == nil {
			log.Printf("W! no pod is found after reading through kubelet, add a placeholder for %s", podKey)
			p.setCachedEntry(podKey, &cachedEntry{creation: time.Now()})
			return false
		}

		// If the entry is not a placeholder, decorate the pod
		if entry.pod.Name == "" {
			p.logger.Warn("no pod information is found in podstore for pod " + podKey)
			return false
		}
		p.decorateCPU(metric, &entry.pod)
		p.decorateMem(metric, &entry.pod)
		p.addStatus(metric, &entry.pod)
		addContainerCount(metric, &entry.pod)
		addContainerID(&entry.pod, metric, kubernetesBlob, p.logger)
		p.addPodOwnersAndPodName(metric, &entry.pod, kubernetesBlob)
		addLabels(&entry.pod, kubernetesBlob)
	}
	return true
}

func (p *PodStore) getCachedEntry(podKey string) *cachedEntry {
	p.Lock()
	defer p.Unlock()
	if content, ok := p.cache.Get(podKey); ok {
		return content.(*cachedEntry)
	}
	return nil
}

func (p *PodStore) setCachedEntry(podKey string, entry *cachedEntry) {
	p.Lock()
	defer p.Unlock()
	p.cache.Set(podKey, entry)
}

func (p *PodStore) refresh(ctx context.Context, now time.Time) {
	var podList []corev1.Pod
	var err error
	doRefresh := func() {
		podList, err = p.podClient.ListPods()
		if err != nil {
			p.logger.Error("fail to get pod from kubelet", zap.Error(err))
		}
	}
	refreshWithTimeout(ctx, doRefresh, refreshInterval)
	p.refreshInternal(now, podList)
}

func (p *PodStore) refreshInternal(now time.Time, podList []corev1.Pod) {
	var podCount int
	var containerCount int
	var cpuRequest uint64
	var memRequest uint64

	for i := range podList {
		pod := podList[i]
		podKey := createPodKeyFromMetaData(&pod)
		if podKey == "" {
			p.logger.Warn("podKey is unavailable, refresh pod store for pod " + pod.Name)
			continue
		}
		if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			tmpCPUReq, _ := getResourceSettingForPod(&pod, p.nodeInfo.getCPUCapacity(), cpuKey, getRequestForContainer)
			cpuRequest += tmpCPUReq
			tmpMemReq, _ := getResourceSettingForPod(&pod, p.nodeInfo.getMemCapacity(), memoryKey, getRequestForContainer)
			memRequest += tmpMemReq
		}
		if pod.Status.Phase == corev1.PodRunning {
			podCount++
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Running != nil {
				containerCount++
			}
		}

		p.setCachedEntry(podKey, &cachedEntry{
			pod:      pod,
			creation: now,
		})
	}

	p.nodeInfo.setNodeStats(nodeStats{podCnt: podCount, containerCnt: containerCount, memReq: memRequest, cpuReq: cpuRequest})
}

func (p *PodStore) decorateNode(metric CIMetric) {
	nodeStats := p.nodeInfo.getNodeStats()

	if metric.HasField(ci.MetricName(ci.TypeNode, ci.CPUTotal)) {
		cpuLimitMetric := ci.MetricName(ci.TypeNode, ci.CPULimit)
		if metric.HasField(cpuLimitMetric) {
			p.nodeInfo.setCPUCapacity(metric.GetField(cpuLimitMetric))
		}
		metric.AddField(ci.MetricName(ci.TypeNode, ci.CPURequest), nodeStats.cpuReq)
		if p.nodeInfo.getCPUCapacity() != 0 {
			metric.AddField(ci.MetricName(ci.TypeNode, ci.CPUReservedCapacity),
				float64(nodeStats.cpuReq)/float64(p.nodeInfo.getCPUCapacity())*100)
		}
	}

	if metric.HasField(ci.MetricName(ci.TypeNode, ci.MemWorkingset)) {
		memLimitMetric := ci.MetricName(ci.TypeNode, ci.MemLimit)
		if metric.HasField(memLimitMetric) {
			p.nodeInfo.setMemCapacity(metric.GetField(memLimitMetric))
		}
		metric.AddField(ci.MetricName(ci.TypeNode, ci.MemRequest), nodeStats.memReq)
		if p.nodeInfo.getMemCapacity() != 0 {
			metric.AddField(ci.MetricName(ci.TypeNode, ci.MemReservedCapacity),
				float64(nodeStats.memReq)/float64(p.nodeInfo.getMemCapacity())*100)
		}
	}

	metric.AddField(ci.MetricName(ci.TypeNode, ci.RunningPodCount), nodeStats.podCnt)
	metric.AddField(ci.MetricName(ci.TypeNode, ci.RunningContainerCount), nodeStats.containerCnt)
}

func (p *PodStore) decorateCPU(metric CIMetric, pod *corev1.Pod) {
	if metric.GetTag(ci.MetricType) == ci.TypePod {
		// add cpu limit and request for pod cpu
		if metric.HasField(ci.MetricName(ci.TypePod, ci.CPUTotal)) {
			podCPUTotal := metric.GetField(ci.MetricName(ci.TypePod, ci.CPUTotal))
			podCPUReq, _ := getResourceSettingForPod(pod, p.nodeInfo.getCPUCapacity(), cpuKey, getRequestForContainer)
			// set podReq to the sum of containerReq which has req
			if podCPUReq != 0 {
				metric.AddField(ci.MetricName(ci.TypePod, ci.CPURequest), podCPUReq)
			}

			if p.nodeInfo.getCPUCapacity() != 0 {
				if podCPUReq != 0 {
					metric.AddField(ci.MetricName(ci.TypePod, ci.CPUReservedCapacity), float64(podCPUReq)/float64(p.nodeInfo.getCPUCapacity())*100)
				}
			}

			podCPULimit, ok := getResourceSettingForPod(pod, p.nodeInfo.getCPUCapacity(), cpuKey, getLimitForContainer)
			// only set podLimit when all the containers has limit
			if ok && podCPULimit != 0 {
				metric.AddField(ci.MetricName(ci.TypePod, ci.CPULimit), podCPULimit)
				metric.AddField(ci.MetricName(ci.TypePod, ci.CPUUtilizationOverPodLimit), podCPUTotal.(float64)/float64(podCPULimit)*100)
			}
		}
	} else if metric.GetTag(ci.MetricType) == ci.TypeContainer {
		// add cpu limit and request for container
		if metric.HasField(ci.MetricName(ci.TypeContainer, ci.CPUTotal)) {
			if containerName := metric.GetTag(ci.ContainerNamekey); containerName != "" {
				for _, containerSpec := range pod.Spec.Containers {
					if containerSpec.Name == containerName {
						if cpuLimit, ok := getLimitForContainer(cpuKey, containerSpec); ok {
							metric.AddField(ci.MetricName(ci.TypeContainer, ci.CPULimit), cpuLimit)
						}
						if cpuReq, ok := getRequestForContainer(cpuKey, containerSpec); ok {
							metric.AddField(ci.MetricName(ci.TypeContainer, ci.CPURequest), cpuReq)
						}
					}
				}
			}
		}
	}
}

func (p *PodStore) decorateMem(metric CIMetric, pod *corev1.Pod) {
	if metric.GetTag(ci.MetricType) == ci.TypePod {
		memWorkingsetMetric := ci.MetricName(ci.TypePod, ci.MemWorkingset)
		if metric.HasField(memWorkingsetMetric) {
			podMemWorkingset := metric.GetField(memWorkingsetMetric)
			// add mem limit and request for pod mem
			podMemReq, _ := getResourceSettingForPod(pod, p.nodeInfo.getMemCapacity(), memoryKey, getRequestForContainer)
			// set podReq to the sum of containerReq which has req
			if podMemReq != 0 {
				metric.AddField(ci.MetricName(ci.TypePod, ci.MemRequest), podMemReq)
			}

			if p.nodeInfo.getMemCapacity() != 0 {
				if podMemReq != 0 {
					metric.AddField(ci.MetricName(ci.TypePod, ci.MemReservedCapacity), float64(podMemReq)/float64(p.nodeInfo.getMemCapacity())*100)
				}
			}

			podMemLimit, ok := getResourceSettingForPod(pod, p.nodeInfo.getMemCapacity(), memoryKey, getLimitForContainer)
			// only set podLimit when all the containers has limit
			if ok && podMemLimit != 0 {
				metric.AddField(ci.MetricName(ci.TypePod, ci.MemLimit), podMemLimit)
				metric.AddField(ci.MetricName(ci.TypePod, ci.MemUtilizationOverPodLimit), float64(podMemWorkingset.(uint64))/float64(podMemLimit)*100)
			}
		}
	} else if metric.GetTag(ci.MetricType) == ci.TypeContainer {
		// add mem limit and request for container
		if metric.HasField(ci.MetricName(ci.TypeContainer, ci.MemWorkingset)) {
			if containerName := metric.GetTag(ci.ContainerNamekey); containerName != "" {
				for _, containerSpec := range pod.Spec.Containers {
					if containerSpec.Name == containerName {
						if memLimit, ok := getLimitForContainer(memoryKey, containerSpec); ok {
							metric.AddField(ci.MetricName(ci.TypeContainer, ci.MemLimit), memLimit)
						}
						if memReq, ok := getRequestForContainer(memoryKey, containerSpec); ok {
							metric.AddField(ci.MetricName(ci.TypeContainer, ci.MemRequest), memReq)
						}
					}
				}
			}
		}
	}
}

func (p *PodStore) addStatus(metric CIMetric, pod *corev1.Pod) {
	if metric.GetTag(ci.MetricType) == ci.TypePod {
		metric.AddTag(ci.PodStatus, string(pod.Status.Phase))
		var curContainerRestarts int
		for _, containerStatus := range pod.Status.ContainerStatuses {
			curContainerRestarts += int(containerStatus.RestartCount)
		}
		podKey := createPodKeyFromMetric(metric)
		if podKey != "" {
			content, ok := p.getPrevMeasurement(ci.TypePod, podKey)
			if ok {
				prevMeasurement := content.(prevPodMeasurement)
				result := 0
				if curContainerRestarts > prevMeasurement.containersRestarts {
					result = curContainerRestarts - prevMeasurement.containersRestarts
				}
				metric.AddField(ci.MetricName(ci.TypePod, ci.ContainerRestartCount), result)
			}
			p.setPrevMeasurement(ci.TypePod, podKey, prevPodMeasurement{containersRestarts: curContainerRestarts})
		}
	} else if metric.GetTag(ci.MetricType) == ci.TypeContainer {
		if containerName := metric.GetTag(ci.ContainerNamekey); containerName != "" {
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.Name == containerName {
					switch {
					case containerStatus.State.Running != nil:
						metric.AddTag(ci.ContainerStatus, "Running")
					case containerStatus.State.Waiting != nil:
						metric.AddTag(ci.ContainerStatus, "Waiting")
						if containerStatus.State.Waiting.Reason != "" {
							metric.AddTag(ci.ContainerStatusReason, containerStatus.State.Waiting.Reason)
						}
					case containerStatus.State.Terminated != nil:
						metric.AddTag(ci.ContainerStatus, "Terminated")
						if containerStatus.State.Terminated.Reason != "" {
							metric.AddTag(ci.ContainerStatusReason, containerStatus.State.Terminated.Reason)
						}
					}

					if containerStatus.LastTerminationState.Terminated != nil && containerStatus.LastTerminationState.Terminated.Reason != "" {
						metric.AddTag(ci.ContainerLastTerminationReason, containerStatus.LastTerminationState.Terminated.Reason)
					}
					containerKey := createContainerKeyFromMetric(metric)
					if containerKey != "" {
						content, ok := p.getPrevMeasurement(ci.TypeContainer, containerKey)
						if ok {
							prevMeasurement := content.(prevContainerMeasurement)
							result := 0
							if int(containerStatus.RestartCount) > prevMeasurement.restarts {
								result = int(containerStatus.RestartCount) - prevMeasurement.restarts
							}
							metric.AddField(ci.ContainerRestartCount, result)
						}
						p.setPrevMeasurement(ci.TypeContainer, containerKey, prevContainerMeasurement{restarts: int(containerStatus.RestartCount)})
					}
				}
			}
		}
	}
}

// It could be used to get limit/request(depend on the passed-in fn) per pod
// return the sum of ResourceSetting and a bool which indicate whether all container set Resource
func getResourceSettingForPod(pod *corev1.Pod, bound uint64, resource corev1.ResourceName, fn func(resource corev1.ResourceName, spec corev1.Container) (uint64, bool)) (uint64, bool) {
	var result uint64
	allSet := true
	for _, containerSpec := range pod.Spec.Containers {
		val, ok := fn(resource, containerSpec)
		if ok {
			result += val
		} else {
			allSet = false
		}
	}
	if bound != 0 && result > bound {
		result = bound
	}
	return result, allSet
}

func getLimitForContainer(resource corev1.ResourceName, spec corev1.Container) (uint64, bool) {
	if v, ok := spec.Resources.Limits[resource]; ok {
		var limit int64
		if resource == cpuKey {
			limit = v.MilliValue()
		} else {
			limit = v.Value()
		}
		// it doesn't make sense for the limits to be negative
		if limit < 0 {
			return 0, false
		}
		return uint64(limit), true
	}
	return 0, false
}

func getRequestForContainer(resource corev1.ResourceName, spec corev1.Container) (uint64, bool) {
	if v, ok := spec.Resources.Requests[resource]; ok {
		var req int64
		if resource == cpuKey {
			req = v.MilliValue()
		} else {
			req = v.Value()
		}
		// it doesn't make sense for the requests to be negative
		if req < 0 {
			return 0, false
		}
		return uint64(req), true
	}
	return 0, false
}

func addContainerID(pod *corev1.Pod, metric CIMetric, kubernetesBlob map[string]any, logger *zap.Logger) {
	if containerName := metric.GetTag(ci.ContainerNamekey); containerName != "" {
		rawID := ""
		for _, container := range pod.Status.ContainerStatuses {
			if metric.GetTag(ci.ContainerNamekey) == container.Name {
				rawID = container.ContainerID
				if rawID != "" {
					ids := strings.Split(rawID, "://")
					if len(ids) == 2 {
						kubernetesBlob[ids[0]] = map[string]string{"container_id": ids[1]}
					} else {
						logger.Warn(fmt.Sprintf("W! Cannot parse container id from %s for container %s", rawID, container.Name))
						kubernetesBlob["container_id"] = rawID
					}
				}
				break
			}
		}
		if rawID == "" {
			kubernetesBlob["container_id"] = metric.GetTag(ci.ContainerIDkey)
		}
		metric.RemoveTag(ci.ContainerIDkey)
	}
}

func addLabels(pod *corev1.Pod, kubernetesBlob map[string]any) {
	labels := make(map[string]string)
	for k, v := range pod.Labels {
		labels[k] = v
	}
	if len(labels) > 0 {
		kubernetesBlob["labels"] = labels
	}
}

func getJobNamePrefix(podName string) string {
	return re.Split(podName, 2)[0]
}

func (p *PodStore) addPodOwnersAndPodName(metric CIMetric, pod *corev1.Pod, kubernetesBlob map[string]any) {
	var owners []any
	podName := ""
	for _, owner := range pod.OwnerReferences {
		if owner.Kind != "" && owner.Name != "" {
			kind := owner.Kind
			name := owner.Name
			switch owner.Kind {
			case ci.ReplicaSet:
				replicaSetClient := p.k8sClient.GetReplicaSetClient()
				rsToDeployment := replicaSetClient.ReplicaSetToDeployment()
				if parent := rsToDeployment[owner.Name]; parent != "" {
					kind = ci.Deployment
					name = parent
				} else if parent := parseDeploymentFromReplicaSet(owner.Name); parent != "" {
					kind = ci.Deployment
					name = parent
				}
			case ci.Job:
				if parent := parseCronJobFromJob(owner.Name); parent != "" {
					kind = ci.CronJob
					name = parent
				} else if !p.prefFullPodName {
					name = getJobNamePrefix(name)
				}
			}
			owners = append(owners, map[string]string{"owner_kind": kind, "owner_name": name})

			if podName == "" {
				switch owner.Kind {
				case ci.StatefulSet:
					podName = pod.Name
				case ci.DaemonSet, ci.Job, ci.ReplicaSet, ci.ReplicationController:
					podName = name
				}
			}
		}
	}

	if len(owners) > 0 {
		kubernetesBlob["pod_owners"] = owners
	}

	// if podName is not set according to a well-known controllers, then set it to its own name
	if podName == "" {
		if strings.HasPrefix(pod.Name, kubeProxy) && !p.prefFullPodName {
			podName = kubeProxy
		} else {
			podName = pod.Name
		}
	}

	metric.AddTag(ci.PodNameKey, podName)
	if p.addFullPodNameMetricLabel {
		metric.AddTag(ci.FullPodNameKey, pod.Name)
	}
}

func addContainerCount(metric CIMetric, pod *corev1.Pod) {
	runningContainerCount := 0
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Running != nil {
			runningContainerCount++
		}
	}
	if metric.GetTag(ci.MetricType) == ci.TypePod {
		metric.AddField(ci.MetricName(ci.TypePod, ci.RunningContainerCount), runningContainerCount)
		metric.AddField(ci.MetricName(ci.TypePod, ci.ContainerCount), len(pod.Status.ContainerStatuses))
	}
}
