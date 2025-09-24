// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipvsreceiver // import "github.com/sergeysedoy97/opentelemetry-collector-contrib/receiver/ipvsreceiver"

import (
	"context"
	"math/bits"
	"strconv"
	"time"

	"github.com/moby/ipvs"
	"github.com/sergeysedoy97/opentelemetry-collector-contrib/receiver/ipvsreceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

type ipvsScraper struct {
	log    *zap.Logger
	mb     *metadata.MetricsBuilder
	handle *ipvs.Handle
}

func newScraper(mbc metadata.MetricsBuilderConfig, set receiver.Settings) *ipvsScraper {
	return &ipvsScraper{
		log:    set.Logger,
		mb:     metadata.NewMetricsBuilder(mbc, set),
		handle: nil,
	}
}

func (s *ipvsScraper) start(context.Context, component.Host) error {
	if s.handle == nil {
		var err error
		s.handle, err = ipvs.New("") // TODO: make the namespace configurable
		return err
	}

	return nil
}

func (s *ipvsScraper) shutdown(context.Context) error {
	if s.handle != nil {
		s.handle.Close()
	}

	return nil
}

func (s *ipvsScraper) scrape(context.Context) (pmetric.Metrics, error) {
	services, err := s.handle.GetServices()
	if err != nil {
		s.log.Error("handle.GetServices:", zap.Error(err))
		return s.mb.Emit(), err
	}

	ts := pcommon.NewTimestampFromTime(time.Now())

	for _, svc := range services {
		if svc.FWMark > 0 {
			continue
		}

		sched := svc.SchedName
		netmask := strconv.Itoa(bits.OnesCount32(svc.Netmask))
		protocol := protocolToString(svc.Protocol)

		vipAddress := svc.Address.String()
		vipAddressFamily := addressFamilyToString(svc.AddressFamily)
		vipPort := strconv.Itoa(int(svc.Port))

		s.mb.RecordIpvsServiceConnectionTotalDataPoint(
			ts,
			int64(svc.Stats.Connections),
			sched,
			netmask,
			protocol,
			vipAddress,
			vipAddressFamily,
			vipPort,
		)
		s.mb.RecordIpvsServicePacketInTotalDataPoint(
			ts,
			int64(svc.Stats.PacketsIn),
			sched,
			netmask,
			protocol,
			vipAddress,
			vipAddressFamily,
			vipPort,
		)
		s.mb.RecordIpvsServicePacketOutTotalDataPoint(
			ts,
			int64(svc.Stats.PacketsOut),
			sched,
			netmask,
			protocol,
			vipAddress,
			vipAddressFamily,
			vipPort,
		)
		s.mb.RecordIpvsServiceInTotalDataPoint(
			ts,
			int64(svc.Stats.BytesIn),
			sched,
			netmask,
			protocol,
			vipAddress,
			vipAddressFamily,
			vipPort,
		)
		s.mb.RecordIpvsServiceOutTotalDataPoint(
			ts,
			int64(svc.Stats.BytesOut),
			sched,
			netmask,
			protocol,
			vipAddress,
			vipAddressFamily,
			vipPort,
		)
		s.mb.RecordIpvsServicePacketInRateDataPoint(
			ts,
			int64(svc.Stats.PPSIn),
			sched,
			netmask,
			protocol,
			vipAddress,
			vipAddressFamily,
			vipPort,
		)
		s.mb.RecordIpvsServicePacketOutRateDataPoint(
			ts,
			int64(svc.Stats.PPSOut),
			sched,
			netmask,
			protocol,
			vipAddress,
			vipAddressFamily,
			vipPort,
		)
		s.mb.RecordIpvsServiceInRateDataPoint(
			ts,
			int64(svc.Stats.BPSIn),
			sched,
			netmask,
			protocol,
			vipAddress,
			vipAddressFamily,
			vipPort,
		)
		s.mb.RecordIpvsServiceOutRateDataPoint(
			ts,
			int64(svc.Stats.BPSOut),
			sched,
			netmask,
			protocol,
			vipAddress,
			vipAddressFamily,
			vipPort,
		)
		s.mb.RecordIpvsServiceConnectionRateDataPoint(
			ts,
			int64(svc.Stats.CPS),
			sched,
			netmask,
			protocol,
			vipAddress,
			vipAddressFamily,
			vipPort,
		)

		destinations, err := s.handle.GetDestinations(svc)
		if err != nil {
			s.log.Warn("handle.GetDestinations:", zap.Error(err))
			continue
		}

		for _, dst := range destinations {
			ripAddress := dst.Address.String()
			ripAddressFamily := addressFamilyToString(dst.AddressFamily)
			ripPort := strconv.Itoa(int(dst.Port))

			s.mb.RecordIpvsDestinationConnectionWeightDataPoint(
				ts,
				int64(dst.Weight),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
			s.mb.RecordIpvsDestinationConnectionActiveCountDataPoint(
				ts,
				int64(dst.ActiveConnections),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
			s.mb.RecordIpvsDestinationConnectionInactiveCountDataPoint(
				ts,
				int64(dst.InactiveConnections),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
			s.mb.RecordIpvsDestinationConnectionTotalDataPoint(
				ts,
				int64(dst.Stats.Connections),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
			s.mb.RecordIpvsDestinationPacketInTotalDataPoint(
				ts,
				int64(dst.Stats.PacketsIn),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
			s.mb.RecordIpvsDestinationPacketOutTotalDataPoint(
				ts,
				int64(dst.Stats.PacketsOut),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
			s.mb.RecordIpvsDestinationInTotalDataPoint(
				ts,
				int64(dst.Stats.BytesIn),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
			s.mb.RecordIpvsDestinationOutTotalDataPoint(
				ts,
				int64(dst.Stats.BytesOut),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
			s.mb.RecordIpvsDestinationPacketInRateDataPoint(
				ts,
				int64(dst.Stats.PPSIn),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
			s.mb.RecordIpvsDestinationPacketOutRateDataPoint(
				ts,
				int64(dst.Stats.PPSOut),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
			s.mb.RecordIpvsDestinationInRateDataPoint(
				ts,
				int64(dst.Stats.BPSIn),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
			s.mb.RecordIpvsDestinationOutRateDataPoint(
				ts,
				int64(dst.Stats.BPSOut),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
			s.mb.RecordIpvsDestinationConnectionRateDataPoint(
				ts,
				int64(dst.Stats.CPS),
				sched,
				netmask,
				protocol,
				vipAddress,
				vipAddressFamily,
				vipPort,
				ripAddress,
				ripAddressFamily,
				ripPort,
			)
		}
	}

	return s.mb.Emit(), nil
}

func protocolToString(p uint16) string {
	switch p {
	case unix.IPPROTO_TCP:
		return "tcp"
	case unix.IPPROTO_UDP:
		return "udp"
	case unix.IPPROTO_SCTP:
		return "sctp"
	default:
		return strconv.FormatUint(uint64(p), 10)
	}
}

func addressFamilyToString(af uint16) string {
	switch af {
	case unix.AF_INET:
		return "inet"
	case unix.AF_INET6:
		return "inet6"
	default:
		return strconv.FormatUint(uint64(af), 10)
	}
}
