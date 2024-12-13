// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package efa // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/efa"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

const (
	defaultCollectionInterval = 20 * time.Second
)

const (
	efaPath            = "/sys/class/infiniband"
	efaK8sResourceName = "vpc.amazonaws.com/efa"

	// hardware counter names
	counterRdmaReadBytes      = "rdma_read_bytes"
	counterRdmaWriteBytes     = "rdma_write_bytes"
	counterRdmaWriteRecvBytes = "rdma_write_recv_bytes"
	counterRxBytes            = "rx_bytes"
	counterRxDrops            = "rx_drops"
	counterTxBytes            = "tx_bytes"
)

var counterNames = map[string]any{
	counterRdmaReadBytes:      nil,
	counterRdmaWriteBytes:     nil,
	counterRdmaWriteRecvBytes: nil,
	counterRxBytes:            nil,
	counterRxDrops:            nil,
	counterTxBytes:            nil,
}

type Scraper struct {
	collectionInterval time.Duration
	cancel             context.CancelFunc

	sysFsReader       sysFsReader
	deltaCalculator   metrics.MetricCalculator
	decorator         stores.Decorator
	podResourcesStore podResourcesStore
	store             *efaStore
	logger            *zap.Logger
}

type sysFsReader interface {
	EfaDataExists() (bool, error)
	ListDevices() ([]efaDeviceName, error)
	ListPorts(deviceName efaDeviceName) ([]string, error)
	ReadCounter(deviceName efaDeviceName, port string, counter string) (uint64, error)
}

type podResourcesStore interface {
	AddResourceName(resourceName string)
	GetContainerInfo(deviceID string, resourceName string) *stores.ContainerInfo
}

type efaStore struct {
	timestamp time.Time
	devices   *efaDevices
}

// efaDevices is a collection of every Amazon Elastic Fabric Adapter (EFA) device in
// /sys/class/infiniband.
type efaDevices map[efaDeviceName]*efaCounters

type efaDeviceName string

// efaCounters contains counter values from files in
// /sys/class/infiniband/<Name>/ports/<Port>/hw_counters
// for a single port of one Amazon Elastic Fabric Adapter device.
type efaCounters struct {
	rdmaReadBytes      uint64 // hw_counters/rdma_read_bytes
	rdmaWriteBytes     uint64 // hw_counters/rdma_write_bytes
	rdmaWriteRecvBytes uint64 // hw_counters/rdma_write_recv_bytes
	rxBytes            uint64 // hw_counters/rx_bytes
	rxDrops            uint64 // hw_counters/rx_drops
	txBytes            uint64 // hw_counters/tx_bytes
}

func NewEfaSyfsScraper(logger *zap.Logger, decorator stores.Decorator, podResourcesStore podResourcesStore) *Scraper {
	ctx, cancel := context.WithCancel(context.Background())
	podResourcesStore.AddResourceName(efaK8sResourceName)
	e := &Scraper{
		collectionInterval: defaultCollectionInterval,
		cancel:             cancel,
		sysFsReader:        defaultSysFsReader(logger),
		deltaCalculator:    metrics.NewMetricCalculator(calculateDelta),
		decorator:          decorator,
		podResourcesStore:  podResourcesStore,
		store:              new(efaStore),
		logger:             logger,
	}

	go e.startScrape(ctx)

	return e
}

func calculateDelta(prev *metrics.MetricValue, val any, _ time.Time) (any, bool) {
	if prev == nil {
		return 0, false
	}
	return val.(uint64) - prev.RawValue.(uint64), true
}

func (s *Scraper) Shutdown() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Scraper) GetMetrics() []pmetric.Metrics {
	var result []pmetric.Metrics

	store := s.store
	if store == nil || store.devices == nil {
		return result
	}
	for deviceName, counters := range *store.devices {
		if counters == nil {
			continue
		}
		containerInfo := s.podResourcesStore.GetContainerInfo(string(deviceName), efaK8sResourceName)

		nodeMetric := stores.NewCIMetric(ci.TypeNodeEFA, s.logger)
		var containerMetric, podMetric stores.CIMetric
		if containerInfo != nil {
			containerMetric = stores.NewCIMetric(ci.TypeContainerEFA, s.logger)
			podMetric = stores.NewCIMetric(ci.TypePodEFA, s.logger)
		}

		measurementValue := map[string]uint64{
			ci.EfaRdmaReadBytes:      counters.rdmaReadBytes,
			ci.EfaRdmaWriteBytes:     counters.rdmaWriteBytes,
			ci.EfaRdmaWriteRecvBytes: counters.rdmaWriteRecvBytes,
			ci.EfaRxBytes:            counters.rxBytes,
			ci.EfaRxDropped:          counters.rxDrops,
			ci.EfaTxBytes:            counters.txBytes,
		}

		for measurement, value := range measurementValue {
			nodeKey := metrics.Key{MetricMetadata: metadata{
				measurement: measurement,
				deviceName:  string(deviceName),
			}}
			deltaVal, found := s.deltaCalculator.Calculate(nodeKey, value, store.timestamp)
			if found {
				nodeMetric.AddField(ci.MetricName(ci.TypeNodeEFA, measurement), deltaVal)
			}

			if containerInfo != nil {
				containerKey := metrics.Key{MetricMetadata: metadata{
					measurement:       measurement,
					deviceName:        string(deviceName),
					containerMetadata: *containerInfo,
				}}
				deltaVal, found := s.deltaCalculator.Calculate(containerKey, value, store.timestamp)
				if found {
					podMetric.AddField(ci.MetricName(ci.TypePodEFA, measurement), deltaVal)
					containerMetric.AddField(ci.MetricName(ci.TypeContainerEFA, measurement), deltaVal)
				}
			}
		}

		allMetrics := make([]stores.CIMetric, 0)
		podContainerMetrics := make([]stores.CIMetric, 0)
		allMetrics = append(allMetrics, nodeMetric)
		if containerInfo != nil {
			allMetrics = append(allMetrics, podMetric, containerMetric)
			podContainerMetrics = append(podContainerMetrics, podMetric, containerMetric)
		}

		for _, m := range allMetrics {
			m.AddTag(ci.EfaDevice, string(deviceName))
			m.AddTag(ci.Timestamp, strconv.FormatInt(store.timestamp.UnixNano(), 10))
		}
		for _, m := range podContainerMetrics {
			m.AddTag(ci.K8sNamespace, containerInfo.Namespace)
			m.AddTag(ci.PodNameKey, containerInfo.PodName)
			m.AddTag(ci.ContainerNamekey, containerInfo.ContainerName)
		}

		for _, m := range allMetrics {
			if len(m.GetFields()) == 0 {
				continue
			}
			metric := s.decorator.Decorate(m)
			result = append(result, ci.ConvertToOTLPMetrics(metric.GetFields(), metric.GetTags(), s.logger))
		}
	}

	return result
}

type metadata struct {
	measurement       string
	deviceName        string
	containerMetadata stores.ContainerInfo
}

func (s *Scraper) startScrape(ctx context.Context) {
	ticker := time.NewTicker(s.collectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := s.scrape()
			if err != nil {
				s.logger.Warn("Failed to scrape EFA metrics from filesystem", zap.Error(err))
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scraper) scrape() error {
	exists, err := s.sysFsReader.EfaDataExists()
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	timestamp := time.Now()

	devices, err := s.parseEfaDevices()
	if err != nil {
		return err
	}

	s.store = &efaStore{
		timestamp: timestamp,
		devices:   devices,
	}

	return nil
}

func (s *Scraper) parseEfaDevices() (*efaDevices, error) {
	deviceNames, err := s.sysFsReader.ListDevices()
	if err != nil {
		return nil, err
	}

	devices := make(efaDevices, len(deviceNames))
	for _, name := range deviceNames {
		counters, err := s.parseEfaDevice(name)
		if err != nil {
			return nil, err
		}

		devices[name] = counters
	}

	return &devices, nil
}

func (s *Scraper) parseEfaDevice(deviceName efaDeviceName) (*efaCounters, error) {
	ports, err := s.sysFsReader.ListPorts(deviceName)
	if err != nil {
		return nil, err
	}

	counters := new(efaCounters)
	for _, port := range ports {
		err := s.readCounters(deviceName, port, counters)
		if err != nil {
			return nil, err
		}
	}
	return counters, nil
}

func (s *Scraper) readCounters(deviceName efaDeviceName, port string, counters *efaCounters) error {
	var errs error
	reader := func(counter string) uint64 {
		value, err := s.sysFsReader.ReadCounter(deviceName, port, counter)
		if err != nil {
			errs = errors.Join(errs, err)
			return 0
		}
		return value
	}

	for counter := range counterNames {
		switch counter {
		case counterRdmaReadBytes:
			counters.rdmaReadBytes += reader(counter)
		case counterRdmaWriteBytes:
			counters.rdmaWriteBytes += reader(counter)
		case counterRdmaWriteRecvBytes:
			counters.rdmaWriteRecvBytes += reader(counter)
		case counterRxBytes:
			counters.rxBytes += reader(counter)
		case counterRxDrops:
			counters.rxDrops += reader(counter)
		case counterTxBytes:
			counters.txBytes += reader(counter)
		}
	}

	return errs
}

func defaultSysFsReader(logger *zap.Logger) sysFsReader {
	return &sysfsReaderImpl{
		logger: logger,
	}
}

type sysfsReaderImpl struct {
	logger *zap.Logger
}

var _ sysFsReader = (*sysfsReaderImpl)(nil)

func (r *sysfsReaderImpl) EfaDataExists() (bool, error) {
	info, err := os.Stat(efaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	err = checkPermissions(info)
	if err != nil {
		r.logger.Warn("not reading from EFA directory", zap.String("path", efaPath), zap.Error(err))
		return false, nil
	}

	return true, nil
}

func (r *sysfsReaderImpl) ListDevices() ([]efaDeviceName, error) {
	dirs, err := os.ReadDir(efaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list EFA devices at %q: %w", efaPath, err)
	}

	result := make([]efaDeviceName, 0)
	for _, dir := range dirs {
		if !dir.IsDir() && dir.Type()&os.ModeSymlink == 0 {
			continue
		}
		result = append(result, efaDeviceName(dir.Name()))
	}

	return result, nil
}

func (r *sysfsReaderImpl) ListPorts(deviceName efaDeviceName) ([]string, error) {
	portsPath := filepath.Join(efaPath, string(deviceName), "ports")
	portDirs, err := os.ReadDir(portsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list EFA ports at %q: %w", portsPath, err)
	}

	result := make([]string, 0)
	for _, dir := range portDirs {
		if !dir.IsDir() {
			continue
		}
		result = append(result, dir.Name())
	}

	return result, nil
}

func (r *sysfsReaderImpl) ReadCounter(deviceName efaDeviceName, port string, counter string) (uint64, error) {
	path := filepath.Join(efaPath, string(deviceName), "ports", port, "hw_counters", counter)
	return readUint64ValueFromFile(path)
}

func readUint64ValueFromFile(path string) (uint64, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) || err.Error() == "operation not supported" || err.Error() == "invalid argument" {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read file %q: %w", path, err)
	}
	stringValue := strings.TrimSpace(string(bytes))

	// Ugly workaround for handling https://github.com/prometheus/node_exporter/issues/966
	// when counters are `N/A (not available)`.
	// This was already patched and submitted, see
	// https://www.spinics.net/lists/linux-rdma/msg68596.html
	// Remove this as soon as the fix lands in the enterprise distros.
	if strings.Contains(stringValue, "N/A (no PMA)") {
		return 0, nil
	}

	value, err := parseUInt64(stringValue)
	if err != nil {
		return 0, err
	}

	return value, nil
}

// Parse string to UInt64
func parseUInt64(value string) (uint64, error) {
	// A base value of zero makes ParseUint infer the correct base using the
	// string's prefix, if any.
	const base = 0
	v, err := strconv.ParseUint(value, base, 64)
	if err != nil {
		return 0, err
	}
	return v, err
}
