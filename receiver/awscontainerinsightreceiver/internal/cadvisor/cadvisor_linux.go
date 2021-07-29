// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build linux

package cadvisor

import (
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
	"go.opentelemetry.io/collector/model/pdata"
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
type createCadvisorManager func(*memory.InMemoryCache, sysfs.SysFs, manager.HouskeepingConfig, cadvisormetrics.MetricSet, *http.Client,
	[]string, string) (cadvisorManager, error)

// this is the default function that are used in production code to create a cadvisor manager
// We define defaultCreateManager and use it as a field value for Cadvisor mainly for the purpose of
// unit testing. Ideally we should just replace the cadvisor manager with a mock in unit tests instead of calling
// createCadvisorManager() to create a mock. However this means that we will not be able
// to unit test the code in `initManager(...)`. For now, I will leave code as it is. Hopefully we can find
// a better way to mock the cadvisor related part in the future.
var defaultCreateManager = func(memoryCache *memory.InMemoryCache, sysfs sysfs.SysFs, houskeepingConfig manager.HouskeepingConfig,
	includedMetricsSet cadvisormetrics.MetricSet, collectorHTTPClient *http.Client, rawContainerCgroupPathPrefixWhiteList []string,
	perfEventsFile string) (cadvisorManager, error) {
	return manager.New(memoryCache, sysfs, houskeepingConfig, includedMetricsSet, collectorHTTPClient, rawContainerCgroupPathPrefixWhiteList, perfEventsFile)
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

type Decorator interface {
	Decorate(*extractors.CAdvisorMetric) *extractors.CAdvisorMetric
}

type Cadvisor struct {
	logger                *zap.Logger
	nodeName              string //get the value from downward API
	createCadvisorManager createCadvisorManager
	manager               cadvisorManager
	version               string
	hostInfo              hostInfo
	k8sDecorator          Decorator
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
	if nodeName == "" {
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

func (c *Cadvisor) decorateMetrics(cadvisormetrics []*extractors.CAdvisorMetric) []*extractors.CAdvisorMetric {
	ebsVolumeIdsUsedAsPV := c.hostInfo.ExtractEbsIDsUsedByKubernetes()
	var result []*extractors.CAdvisorMetric
	for _, m := range cadvisormetrics {
		tags := m.GetTags()
		c.addEbsVolumeInfo(tags, ebsVolumeIdsUsedAsPV)

		//add version
		tags[ci.Version] = c.version

		//add NodeName for node, pod and container
		metricType := tags[ci.MetricType]
		if c.nodeName != "" && (ci.IsNode(metricType) || ci.IsInstance(metricType) ||
			ci.IsPod(metricType) || ci.IsContainer(metricType)) {
			tags[ci.NodeNameKey] = c.nodeName
		}

		//add instance id and type
		if instanceID := c.hostInfo.GetInstanceID(); instanceID != "" {
			tags[ci.InstanceID] = instanceID
		}
		if instanceType := c.hostInfo.GetInstanceType(); instanceType != "" {
			tags[ci.InstanceType] = instanceType
		}

		//add cluster name and auto scaling group name
		tags[ci.ClusterNameKey] = c.hostInfo.GetClusterName()
		tags[ci.AutoScalingGroupNameKey] = c.hostInfo.GetAutoScalingGroupName()
		out := c.k8sDecorator.Decorate(m)
		if out != nil {
			result = append(result, out)
		}
	}

	return result
}

// GetMetrics generates metrics from cadvisor
func (c *Cadvisor) GetMetrics() []pdata.Metrics {
	c.logger.Debug("collect data from cadvisor...")
	var result []pdata.Metrics
	var containerinfos []*cInfo.ContainerInfo
	var err error

	//don't emit metrics if the cluster name is not detected
	clusterName := c.hostInfo.GetClusterName()
	if clusterName == "" {
		c.logger.Warn("Failed to detect cluster name. Drop all metrics")
		return result
	}

	req := &cInfo.ContainerInfoRequest{
		NumStats: 1,
	}

	containerinfos, err = c.manager.SubcontainersInfo("/", req)
	if err != nil {
		c.logger.Warn("GetContainerInfo failed", zap.Error(err))
		return result
	}

	c.logger.Debug("cadvisor containers stats", zap.Int("size", len(containerinfos)))
	out := processContainers(containerinfos, c.hostInfo, c.containerOrchestrator, c.logger)

	results := c.decorateMetrics(out)

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

	houseKeepingConfig := manager.HouskeepingConfig{
		Interval:     &maxHousekeepingInterval,
		AllowDynamic: &allowDynamicHousekeeping,
	}
	// Create and start the cAdvisor container manager.
	m, err := createManager(memory.New(statsCacheDuration, nil), sysFs, houseKeepingConfig, includedMetrics, http.DefaultClient, cgroupRoots, "")
	if err != nil {
		c.logger.Error("cadvisor manager allocate failed, ", zap.Error(err))
		return err
	}
	cadvisormetrics.RegisterPlugin("containerd", containerd.NewPlugin())
	cadvisormetrics.RegisterPlugin("crio", crio.NewPlugin())
	cadvisormetrics.RegisterPlugin("docker", docker.NewPlugin())
	cadvisormetrics.RegisterPlugin("systemd", systemd.NewPlugin())
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
