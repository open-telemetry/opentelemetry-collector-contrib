// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver"

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver/internal/metadata"
)

const (
	defaultQueryTimeout    = 5 * time.Second
	defaultRecordType      = "A"
	defaultNetworkProtocol = "udp"
)

type scraper struct {
	cfg      *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
	exchange func(ctx context.Context, client *dns.Client, msg *dns.Msg, addr string) (*dns.Msg, time.Duration, error)
}

func exchange(ctx context.Context, client *dns.Client, msg *dns.Msg, addr string) (*dns.Msg, time.Duration, error) {
	return client.ExchangeContext(ctx, msg, addr)
}

func newScraper(cfg *Config, settings receiver.Settings) *scraper {
	return &scraper{
		cfg:      cfg,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		exchange: exchange,
	}
}

// check represents a single (dns server, hostname) query to be performed.
type check struct {
	server   DNSServerConfig
	hostname HostnameConfig
}

// checkResult holds the outcome of a single check, to be recorded into the
// MetricsBuilder sequentially.
// This is needed since MetricsBuilder is not safe for concurrent use.
type checkResult struct {
	check
	recordType string
	now        pcommon.Timestamp
	resp       *dns.Msg
	rtt        time.Duration
	err        error
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var checks []check
	for _, server := range s.cfg.DNSServers {
		for _, hostname := range s.cfg.Hostnames {
			checks = append(checks, check{server: server, hostname: hostname})
		}
	}

	results := make([]checkResult, len(checks))
	var wg sync.WaitGroup
	wg.Add(len(checks))
	for i, c := range checks {
		go func(i int, c check) {
			defer wg.Done()
			results[i] = s.exchangeCheck(ctx, c)
		}(i, c)
	}
	wg.Wait()

	errs := &scrapererror.ScrapeErrors{}
	for i := range results {
		if results[i].err != nil {
			errs.AddPartial(1, results[i].err)
		}
		s.recordResult(&results[i])
	}

	metrics := s.mb.Emit()
	removeAbsentDNSAttributes(metrics)
	return metrics, errs.Combine()
}

// noRcode is the sentinel dns.rcode value recorded when no response was
// received at all, so that removeAbsentDNSAttributes can strip the attribute.
const noRcode = -1

var (
	dnsRcodeAttr          = string(metadata.DnscheckStatusMetricAttributeKeyDNSRcode)
	dnsResolvedIPAttr     = string(metadata.DnscheckStatusMetricAttributeKeyDNSResolvedIP)
	dnsResolvedAllIPsAttr = string(metadata.DnscheckStatusMetricAttributeKeyDNSResolvedAllIps)
)

// removeAbsentDNSAttributes strips dnscheck.status attributes that mdatagen
// always writes, for the cases where they should be absent per the metric's
// documented semantics: dns.rcode when no response was received, and
// dns.resolved.ip/dns.resolved.all.ips when no address was resolved (e.g. the
// query failed, or the record type queried isn't an address type).
func removeAbsentDNSAttributes(metrics pmetric.Metrics) {
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				if m.Name() != metadata.MetricsInfo.DnscheckStatus.Name || m.Type() != pmetric.MetricTypeSum {
					continue
				}
				dps := m.Sum().DataPoints()
				for l := 0; l < dps.Len(); l++ {
					dp := dps.At(l)
					if rcode, ok := dp.Attributes().Get(dnsRcodeAttr); ok && rcode.Int() == noRcode {
						dp.Attributes().Remove(dnsRcodeAttr)
					}
					if ip, ok := dp.Attributes().Get(dnsResolvedIPAttr); ok && ip.Str() == "" {
						dp.Attributes().Remove(dnsResolvedIPAttr)
					}
					if ips, ok := dp.Attributes().Get(dnsResolvedAllIPsAttr); ok && ips.Str() == "" {
						dp.Attributes().Remove(dnsResolvedAllIPsAttr)
					}
				}
			}
		}
	}
}

func (s *scraper) exchangeCheck(ctx context.Context, c check) checkResult {
	endpoint := c.server.Endpoint
	if _, _, err := net.SplitHostPort(endpoint); err != nil {
		endpoint = net.JoinHostPort(endpoint, "53")
	}

	network := c.server.Network
	if network == "" {
		network = defaultNetworkProtocol
	}

	timeout := c.server.Timeout
	if timeout <= 0 {
		timeout = defaultQueryTimeout
	}

	recordType := c.hostname.RecordType
	if recordType == "" {
		recordType = defaultRecordType
	}
	qtype := dns.StringToType[strings.ToUpper(recordType)]

	client := &dns.Client{Net: network, Timeout: timeout}
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(c.hostname.Name), qtype)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	now := pcommon.NewTimestampFromTime(time.Now())
	resp, rtt, err := s.exchange(ctx, client, msg, endpoint)

	return checkResult{check: c, recordType: recordType, now: now, resp: resp, rtt: rtt, err: err}
}

func (s *scraper) recordResult(result *checkResult) {
	server := result.server.Endpoint
	domain := result.hostname.Name
	recordType := result.recordType

	defer func() {
		rb := s.mb.NewResourceBuilder()
		rb.SetDNSRecordType(recordType)
		rb.SetDNSDomain(domain)
		rb.SetDNSServer(server)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}()

	if result.err != nil {
		s.mb.RecordDnscheckErrorDataPoint(result.now, 1, result.err.Error())
		s.mb.RecordDnscheckStatusDataPoint(result.now, 0, noRcode, "", "")
		return
	}

	s.mb.RecordDnscheckDurationDataPoint(result.now, result.rtt.Milliseconds())

	var resolvedIP string
	var resolvedAllIPs []string
	for _, rr := range result.resp.Answer {
		var ip string
		switch v := rr.(type) {
		case *dns.A:
			ip = v.A.String()
		case *dns.AAAA:
			ip = v.AAAA.String()
		}
		if ip != "" {
			resolvedAllIPs = append(resolvedAllIPs, ip)
		}
	}
	if len(resolvedAllIPs) > 0 {
		resolvedIP = resolvedAllIPs[0]
	}

	var success int64
	if result.resp.Rcode == dns.RcodeSuccess {
		success = 1
	}

	s.mb.RecordDnscheckStatusDataPoint(result.now, success, int64(result.resp.Rcode), strings.Join(resolvedAllIPs, ","), resolvedIP)
}
