// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsefareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver"

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsefareceiver/internal/metadata"
)

// efaCounter ties a sysfs hw_counter file name to the MetricsBuilder method
// that records it. Defined once so the two can never drift out of sync.
type efaCounter struct {
	name   string // file name under hw_counters/
	record func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, val int64, device, port, eniID string)
}

var efaCounters = []efaCounter{
	{"rdma_read_bytes", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRdmaReadBytesDataPoint(ts, v, device, port, eniID)
	}},
	{"rdma_write_bytes", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRdmaWriteBytesDataPoint(ts, v, device, port, eniID)
	}},
	{"rdma_write_recv_bytes", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRdmaWriteRecvBytesDataPoint(ts, v, device, port, eniID)
	}},
	{"rx_bytes", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRxBytesDataPoint(ts, v, device, port, eniID)
	}},
	{"rx_drops", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRxDroppedDataPoint(ts, v, device, port, eniID)
	}},
	{"tx_bytes", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaTxBytesDataPoint(ts, v, device, port, eniID)
	}},
	{"retrans_bytes", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRetransBytesDataPoint(ts, v, device, port, eniID)
	}},
	{"retrans_pkts", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRetransPktsDataPoint(ts, v, device, port, eniID)
	}},
	{"retrans_timeout_events", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRetransTimeoutEventsDataPoint(ts, v, device, port, eniID)
	}},
	{"unresponsive_remote_events", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaUnresponsiveRemoteEventsDataPoint(ts, v, device, port, eniID)
	}},
	{"impaired_remote_conn_events", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaImpairedRemoteConnEventsDataPoint(ts, v, device, port, eniID)
	}},
	{"tx_pkts", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaTxPktsDataPoint(ts, v, device, port, eniID)
	}},
	{"rx_pkts", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRxPktsDataPoint(ts, v, device, port, eniID)
	}},
	{"send_bytes", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaSendBytesDataPoint(ts, v, device, port, eniID)
	}},
	{"recv_bytes", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRecvBytesDataPoint(ts, v, device, port, eniID)
	}},
	{"send_wrs", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaSendWrsDataPoint(ts, v, device, port, eniID)
	}},
	{"recv_wrs", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRecvWrsDataPoint(ts, v, device, port, eniID)
	}},
	{"rdma_write_wrs", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRdmaWriteWrsDataPoint(ts, v, device, port, eniID)
	}},
	{"rdma_read_wrs", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRdmaReadWrsDataPoint(ts, v, device, port, eniID)
	}},
	{"rdma_write_wr_err", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRdmaWriteWrErrDataPoint(ts, v, device, port, eniID)
	}},
	{"rdma_read_wr_err", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRdmaReadWrErrDataPoint(ts, v, device, port, eniID)
	}},
	{"rdma_read_resp_bytes", func(mb *metadata.MetricsBuilder, ts pcommon.Timestamp, v int64, device, port, eniID string) {
		mb.RecordEfaRdmaReadRespBytesDataPoint(ts, v, device, port, eniID)
	}},
}

type efaScraper struct {
	logger      *zap.Logger
	mb          *metadata.MetricsBuilder
	reader      sysFsReader
	hostPath    string
	eniResolver eniResolver
	eniCache    map[string]string
}

func newScraper(cfg *Config, settings receiver.Settings) *efaScraper {
	return &efaScraper{
		logger:   settings.Logger,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		hostPath: cfg.HostPath,
		eniCache: make(map[string]string),
	}
}

func (s *efaScraper) start(_ context.Context, _ component.Host) error {
	s.reader = newSysFsReader(s.hostPath, s.logger)
	s.eniResolver = newIMDSENIResolver()
	s.logger.Info("Starting AWS EFA receiver", zap.String("host_path", s.hostPath))
	return nil
}

func (s *efaScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	exists, err := s.reader.EfaDataExists()
	if err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("failed to check EFA data: %w", err)
	}
	if !exists {
		s.logger.Debug("No EFA devices found or insufficient permissions, skipping scrape")
		return pmetric.NewMetrics(), nil
	}

	devices, err := s.readAllDevices()
	if err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("failed to read EFA devices: %w", err)
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	for _, dev := range devices {
		s.recordMetrics(now, dev.counters, dev.name, dev.port, dev.eniID)
	}

	return s.mb.Emit(), nil
}

func (s *efaScraper) readAllDevices() ([]efaDevice, error) {
	deviceNames, err := s.reader.ListDevices()
	if err != nil {
		return nil, err
	}

	var devices []efaDevice
	for _, name := range deviceNames {
		ports, err := s.reader.ListPorts(name)
		if err != nil {
			s.logger.Warn("Failed to list ports for EFA device",
				zap.String("device", name), zap.Error(err))
			continue
		}

		eniID := s.resolveENI(name)

		for _, port := range ports {
			counters, err := s.readCounters(name, port)
			if err != nil {
				s.logger.Warn("Partial failure reading counters for EFA device port",
					zap.String("device", name), zap.String("port", port), zap.Error(err))
			}
			if len(counters) == 0 {
				continue
			}
			devices = append(devices, efaDevice{
				name:     name,
				port:     port,
				eniID:    eniID,
				counters: counters,
			})
		}
	}

	return devices, nil
}

// resolveENI resolves the ENI ID for a device via GID → MAC → IMDS lookup.
// Successful results are cached permanently since ENI-to-device mappings don't
// change at runtime. Failures are not cached so transient IMDS issues are
// retried on the next scrape.
func (s *efaScraper) resolveENI(deviceName string) string {
	if eniID, ok := s.eniCache[deviceName]; ok {
		return eniID
	}

	gid, err := s.reader.ReadGID(deviceName)
	if err != nil {
		s.logger.Warn("Failed to read GID for EFA device, emitting metrics without eni_id",
			zap.String("device", deviceName), zap.Error(err))
		return ""
	}

	mac, err := ipv6LinkLocalToMAC(gid)
	if err != nil {
		s.logger.Warn("Failed to convert GID to MAC for EFA device, emitting metrics without eni_id",
			zap.String("device", deviceName), zap.String("gid", gid), zap.Error(err))
		return ""
	}

	eniID, err := s.eniResolver.GetENIID(mac)
	if err != nil {
		s.logger.Warn("Failed to resolve ENI ID from IMDS, emitting metrics without eni_id",
			zap.String("device", deviceName), zap.String("mac", mac), zap.Error(err))
		return ""
	}

	s.logger.Info("Resolved ENI ID for EFA device",
		zap.String("device", deviceName), zap.String("eni_id", eniID))
	s.eniCache[deviceName] = eniID
	return eniID
}

// readCounters reads all known EFA counters for a device-port combination.
// Counters that are missing or unavailable (e.g., on older EFA driver versions)
// are silently skipped. Only unexpected I/O errors are accumulated.
func (s *efaScraper) readCounters(deviceName, port string) (map[string]uint64, error) {
	var errs error
	counters := make(map[string]uint64, len(efaCounters))

	for _, c := range efaCounters {
		value, err := s.reader.ReadCounter(deviceName, port, c.name)
		if err != nil {
			if errors.Is(err, errCounterNotAvailable) {
				continue
			}
			errs = errors.Join(errs, err)
			continue
		}
		counters[c.name] = value
	}

	return counters, errs
}

func (s *efaScraper) recordMetrics(ts pcommon.Timestamp, counters map[string]uint64, device, port, eniID string) {
	for _, c := range efaCounters {
		val, ok := counters[c.name]
		if !ok {
			continue
		}
		if val > math.MaxInt64 {
			s.logger.Warn("Skipping metric, value exceeds int64 range",
				zap.String("counter", c.name), zap.Uint64("value", val))
			continue
		}
		c.record(s.mb, ts, int64(val), device, port, eniID)
	}
}
