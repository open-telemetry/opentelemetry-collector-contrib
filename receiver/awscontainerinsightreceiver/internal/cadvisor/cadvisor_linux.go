// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package cadvisor // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor"

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/google/cadvisor/cache/memory"
	cadvisormetrics "github.com/google/cadvisor/container"
	"github.com/google/cadvisor/container/containerd"
	"github.com/google/cadvisor/container/crio"
	"github.com/google/cadvisor/container/docker"
	"github.com/google/cadvisor/container/systemd"
	cInfo "github.com/google/cadvisor/info/v1"
	"github.com/google/cadvisor/manager"
	"github.com/google/cadvisor/utils/sysfs"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
)

// The amount of time for which to keep stats in memory.
const statsCacheDuration = 2 * time.Minute

// Max collection interval, it is not meaningful if allowDynamicHousekeeping = false
var maxHousekeepingInterval = 15 * time.Second

// When allowDynamicHousekeeping is true, the collection interval is floating between 1s(default) to maxHousekeepingInterval
var allowDynamicHousekeeping = true

const defaultHousekeepingInterval = 10 * time.Second

// define the interface for cadvisor manager so that we can mock it in tests.
// For detailed information about these APIs, see https://github.com/google/cadvisor/blob/release-v0.39/manager/manager.go
type cadvisorManager interface {
	// Start the manager. Calling other manager methods before this returns
	// may produce undefined behavior.
	Start() error

	// Get information about all subcontainers of the specified container (includes self).
	SubcontainersInfo(containerName string, query *cInfo.ContainerInfoRequest) ([]*cInfo.ContainerInfo, error)
}

// define a function type for creating a cadvisor manager
type createCadvisorManager func(*memory.InMemoryCache, sysfs.SysFs, manager.HousekeepingConfig, cadvisormetrics.MetricSet, *http.Client,
	[]string, string) (cadvisorManager, error)

// this is the default function that are used in production code to create a cadvisor manager
// We define defaultCreateManager and use it as a field value for Cadvisor mainly for the purpose of
// unit testing. Ideally we should just replace the cadvisor manager with a mock in unit tests instead of calling
// createCadvisorManager() to create a mock. However this means that we will not be able
// to unit test the code in `initManager(...)`. For now, I will leave code as it is. Hopefully we can find
// a better way to mock the cadvisor related part in the future.
var defaultCreateManager = func(memoryCache *memory.InMemoryCache, sysfs sysfs.SysFs, housekeepingConfig manager.HousekeepingConfig,
	includedMetricsSet cadvisormetrics.MetricSet, collectorHTTPClient *http.Client, rawContainerCgroupPathPrefixWhiteList []string,
	perfEventsFile string) (cadvisorManager, error) {
	return manager.New(memoryCache, sysfs, housekeepingConfig, includedMetricsSet, collectorHTTPClient, rawContainerCgroupPathPrefixWhiteList, []string{}, perfEventsFile, 0)
}

// Option is a function that can be used to configure Cadvisor struct
type Option func(*Cadvisor)

func cadvisorManagerCreator(f createCadvisorManager) Option {
	return func(c *Cadvisor) {
		c.createCadvisorManager = f
	}
}

// WithDecorator constructs an option for configuring the metric decorator
func WithDecorator(d Decorator) Option {
	return func(c *Cadvisor) {
		c.k8sDecorator = d
	}
}

func WithECSInfoCreator(f EcsInfo) Option {
	return func(c *Cadvisor) {
		c.ecsInfo = f
	}
}

type hostInfo interface {
	GetNumCores() int64
	GetMemoryCapacity() int64
	GetClusterName() string
	GetEBSVolumeID(string) string
	ExtractEbsIDsUsedByKubernetes() map[string]string
	GetInstanceID() string
	GetInstanceType() string
	GetAutoScalingGroupName() string
}

type EcsInfo interface {
	GetCPUReserved() int64
	GetMemReserved() int64
	GetRunningTaskCount() int64
	GetContainerInstanceID() string
	GetClusterName() string
}

type Decorator interface {
	Decorate(*extractors.CAdvisorMetric) *extractors.CAdvisorMetric
	Shutdown() error
}

type Cadvisor struct {
	logger                *zap.Logger
	nodeName              string // get the value from downward API
	createCadvisorManager createCadvisorManager
	manager               cadvisorManager
	version               string
	hostInfo              hostInfo
	k8sDecorator          Decorator
	ecsInfo               EcsInfo
	containerOrchestrator string
}

func init() {
	// Override the default cAdvisor housekeeping interval.
	// We should use a proper way to configure once the issue is resolved: https://github.com/google/cadvisor/issues/2886
	*manager.HousekeepingInterval = defaultHousekeepingInterval
}

// New creates a Cadvisor struct which can generate metrics from embedded cadvisor lib
func New(containerOrchestrator string, hostInfo hostInfo, logger *zap.Logger, options ...Option) (*Cadvisor, error) {
	nodeName := os.Getenv("HOST_NAME")
	if nodeName == "" && containerOrchestrator == ci.EKS {
		return nil, errors.New("missing environment variable HOST_NAME. Please check your deployment YAML config")
	}

	c := &Cadvisor{
		logger:                logger,
		nodeName:              nodeName,
		version:               "0",
		createCadvisorManager: defaultCreateManager,
		containerOrchestrator: containerOrchestrator,
	}

	// apply additional options
	for _, option := range options {
		option(c)
	}

	if err := c.initManager(c.createCadvisorManager); err != nil {
		return nil, err
	}

	c.hostInfo = hostInfo
	return c, nil
}

var metricsExtractors = []extractors.MetricExtractor{}

func GetMetricsExtractors() []extractors.MetricExtractor {
	return metricsExtractors
}

func (c *Cadvisor) Shutdown() error {
	var errs error
	for _, ext := range metricsExtractors {
		errs = errors.Join(errs, ext.Shutdown())
	}

	if c.k8sDecorator != nil {
		errs = errors.Join(errs, c.k8sDecorator.Shutdown())
	}
	return errs
}

func (c *Cadvisor) addEbsVolumeInfo(tags map[string]string, ebsVolumeIdsUsedAsPV map[string]string) {
	deviceName, ok := tags[ci.DiskDev]
	if !ok {
		return
	}

	if c.hostInfo != nil {
		if volID := c.hostInfo.GetEBSVolumeID(deviceName); volID != "" {
			tags[ci.HostEbsVolumeID] = volID
		}
	}

	if tags[ci.MetricType] == ci.TypeContainerFS || tags[ci.MetricType] == ci.TypeNodeFS ||
		tags[ci.MetricType] == ci.TypeNodeDiskIO || tags[ci.MetricType] == ci.TypeContainerDiskIO {
		if volID := ebsVolumeIdsUsedAsPV[deviceName]; volID != "" {
			tags[ci.EbsVolumeID] = volID
		}
	}
}

func (c *Cadvisor) addECSMetrics(cadvisormetrics []*extractors.CAdvisorMetric) {

	if len(cadvisormetrics) == 0 {
		c.logger.Warn("cadvisor can't collect any metrics!")
	}

	for _, cadvisormetric := range cadvisormetrics {
		if cadvisormetric.GetMetricType() == ci.TypeInstance {
			metricMap := cadvisormetric.GetFields()
			cpuReserved := c.ecsInfo.GetCPUReserved()
			memReserved := c.ecsInfo.GetMemReserved()
			if cpuReserved == 0 && memReserved == 0 {
				c.logger.Warn("Can't get mem or cpu reserved!")
			}
			cpuLimits, cpuExist := metricMap[ci.MetricName(ci.TypeInstance, ci.CPULimit)]
			memLimits, memExist := metricMap[ci.MetricName(ci.TypeInstance, ci.MemLimit)]

			if !cpuExist && !memExist {
				c.logger.Warn("Can't get mem or cpu limit")
			} else {
				// cgroup standard cpulimits should be cadvisor standard * 1.024
				metricMap[ci.MetricName(ci.TypeInstance, ci.CPUReservedCapacity)] = float64(cpuReserved) / (float64(cpuLimits.(int64)) * 1.024) * 100
				metricMap[ci.MetricName(ci.TypeInstance, ci.MemReservedCapacity)] = float64(memReserved) / float64(memLimits.(int64)) * 100
			}

			if c.ecsInfo.GetRunningTaskCount() == 0 {
				c.logger.Warn("Can't get running task number")
			} else {
				metricMap[ci.MetricName(ci.TypeInstance, ci.RunningTaskCount)] = c.ecsInfo.GetRunningTaskCount()
			}
		}
	}
}

func addECSResources(tags map[string]string) {
	metricType := tags[ci.MetricType]
	if metricType == "" {
		return
	}
	var sources []string
	switch metricType {
	case ci.TypeInstance:
		sources = []string{"cadvisor", "/proc", "ecsagent", "calculated"}
	case ci.TypeInstanceFS:
		sources = []string{"cadvisor", "calculated"}
	case ci.TypeInstanceNet:
		sources = []string{"cadvisor", "calculated"}
	case ci.TypeInstanceDiskIO:
		sources = []string{"cadvisor"}
	}
	if len(sources) > 0 {
		sourcesInfo, err := json.Marshal(sources)
		if err != nil {
			return
		}
		tags[ci.SourcesKey] = string(sourcesInfo)
	}
}

func (c *Cadvisor) decorateMetrics(cadvisormetrics []*extractors.CAdvisorMetric) []*extractors.CAdvisorMetric {
	ebsVolumeIdsUsedAsPV := c.hostInfo.ExtractEbsIDsUsedByKubernetes()
	var result []*extractors.CAdvisorMetric
	for _, m := range cadvisormetrics {
		tags := m.GetTags()
		c.addEbsVolumeInfo(tags, ebsVolumeIdsUsedAsPV)

		// add version
		tags[ci.Version] = c.version

		// add NodeName for node, pod and container
		metricType := tags[ci.MetricType]
		if c.nodeName != "" && (ci.IsNode(metricType) || ci.IsInstance(metricType) ||
			ci.IsPod(metricType) || ci.IsContainer(metricType)) {
			tags[ci.NodeNameKey] = c.nodeName
		}

		// add instance id and type
		if instanceID := c.hostInfo.GetInstanceID(); instanceID != "" {
			tags[ci.InstanceID] = instanceID
		}
		if instanceType := c.hostInfo.GetInstanceType(); instanceType != "" {
			tags[ci.InstanceType] = instanceType
		}

		// add scaling group name
		tags[ci.AutoScalingGroupNameKey] = c.hostInfo.GetAutoScalingGroupName()

		// add ECS cluster name and container instance id
		if c.containerOrchestrator == ci.ECS {
			if c.ecsInfo.GetClusterName() == "" {
				c.logger.Warn("Can't get cluster name")
			} else {
				tags[ci.ClusterNameKey] = c.ecsInfo.GetClusterName()
			}

			if c.ecsInfo.GetContainerInstanceID() == "" {
				c.logger.Warn("Can't get containerInstanceId")
			} else {
				tags[ci.ContainerInstanceIDKey] = c.ecsInfo.GetContainerInstanceID()
			}
			addECSResources(tags)
		}

		// add tags for EKS
		if c.containerOrchestrator == ci.EKS {

			tags[ci.ClusterNameKey] = c.hostInfo.GetClusterName()

			out := c.k8sDecorator.Decorate(m)
			if out != nil {
				result = append(result, out)
			}
		}

	}

	return result
}

// GetMetrics generates metrics from cadvisor
func (c *Cadvisor) GetMetrics() []pmetric.Metrics {
	c.logger.Debug("collect data from cadvisor...")
	var result []pmetric.Metrics
	var containerinfos []*cInfo.ContainerInfo
	var err error

	// For EKS don't emit metrics if the cluster name is not detected
	if c.containerOrchestrator == ci.EKS {
		clusterName := c.hostInfo.GetClusterName()
		if clusterName == "" {
			c.logger.Warn("Failed to detect cluster name. Drop all metrics")
			return result
		}
	}

	req := &cInfo.ContainerInfoRequest{
		NumStats: 1,
	}

	containerinfos, err = c.manager.SubcontainersInfo("/", req)
	if err != nil {
		c.logger.Warn("GetContainerInfo failed", zap.Error(err))
		return result
	}

	out := processContainers(containerinfos, c.hostInfo, c.containerOrchestrator, c.logger)
	results := c.decorateMetrics(out)

	if c.containerOrchestrator == ci.ECS {
		results = out
		c.addECSMetrics(results)
	}

	for _, cadvisorMetric := range results {
		md := ci.ConvertToOTLPMetrics(cadvisorMetric.GetFields(), cadvisorMetric.GetTags(), c.logger)
		result = append(result, md)
	}

	return result
}

// initManager accepts a function of type createCadvisorManager which can be used
// to create a cadvisor manager
func (c *Cadvisor) initManager(createManager createCadvisorManager) error {
	sysFs := sysfs.NewRealSysFs()
	includedMetrics := cadvisormetrics.MetricSet{
		cadvisormetrics.CpuUsageMetrics:     struct{}{},
		cadvisormetrics.MemoryUsageMetrics:  struct{}{},
		cadvisormetrics.DiskIOMetrics:       struct{}{},
		cadvisormetrics.NetworkUsageMetrics: struct{}{},
		cadvisormetrics.DiskUsageMetrics:    struct{}{},
	}
	var cgroupRoots []string
	if c.containerOrchestrator == ci.EKS {
		cgroupRoots = []string{"/kubepods"}
	}

	houseKeepingConfig := manager.HousekeepingConfig{
		Interval:     &maxHousekeepingInterval,
		AllowDynamic: &allowDynamicHousekeeping,
	}
	// Create and start the cAdvisor container manager.
	m, err := createManager(memory.New(statsCacheDuration, nil), sysFs, houseKeepingConfig, includedMetrics, http.DefaultClient, cgroupRoots, "")
	if err != nil {
		c.logger.Error("cadvisor manager allocate failed, ", zap.Error(err))
		return err
	}
	_ = cadvisormetrics.RegisterPlugin("containerd", containerd.NewPlugin())
	_ = cadvisormetrics.RegisterPlugin("crio", crio.NewPlugin())
	_ = cadvisormetrics.RegisterPlugin("docker", docker.NewPlugin())
	_ = cadvisormetrics.RegisterPlugin("systemd", systemd.NewPlugin())
	c.manager = m
	err = c.manager.Start()
	if err != nil {
		c.logger.Error("cadvisor manager start failed", zap.Error(err))
		return err
	}

	metricsExtractors = []extractors.MetricExtractor{}
	metricsExtractors = append(metricsExtractors, extractors.NewCPUMetricExtractor(c.logger))
	metricsExtractors = append(metricsExtractors, extractors.NewMemMetricExtractor(c.logger))
	metricsExtractors = append(metricsExtractors, extractors.NewDiskIOMetricExtractor(c.logger))
	metricsExtractors = append(metricsExtractors, extractors.NewNetMetricExtractor(c.logger))
	metricsExtractors = append(metricsExtractors, extractors.NewFileSystemMetricExtractor(c.logger))

	return nil
}
