// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver"

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver/internal/metadata"
)

// startTestDNSServer starts an in-process DNS server on a random UDP port that
// answers queries for "example.com." with a single A record, and returns
// NXDOMAIN for anything else.
func startTestDNSServer(t *testing.T) string {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)

	mux := dns.NewServeMux()
	mux.HandleFunc("example.com.", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		switch r.Question[0].Qtype {
		case dns.TypeA:
			rr, err := dns.NewRR("example.com. 60 IN A 93.184.216.34")
			require.NoError(t, err)
			m.Answer = append(m.Answer, rr)
		case dns.TypeMX:
			rr, err := dns.NewRR("example.com. 60 IN MX 10 mail.example.com.")
			require.NoError(t, err)
			m.Answer = append(m.Answer, rr)
		}
		_ = w.WriteMsg(m)
	})
	mux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeNameError)
		_ = w.WriteMsg(m)
	})

	server := &dns.Server{PacketConn: pc, Handler: mux}
	started := make(chan struct{})
	server.NotifyStartedFunc = func() { close(started) }

	go func() {
		_ = server.ActivateAndServe()
	}()
	t.Cleanup(func() { _ = server.Shutdown() })
	<-started

	return pc.LocalAddr().String()
}

func TestScrapeSuccess(t *testing.T) {
	addr := startTestDNSServer(t)

	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		DNSServers:           []DNSServerConfig{{Endpoint: addr, Timeout: 2 * time.Second}},
		Hostnames:            []HostnameConfig{{Name: "example.com", RecordType: "A"}},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	status := findMetric(t, metrics, "dnscheck.status")
	dp := status.Sum().DataPoints().At(0)
	require.Equal(t, int64(1), dp.IntValue())

	rcode, ok := dp.Attributes().Get("dns.rcode")
	require.True(t, ok)
	require.Equal(t, int64(dns.RcodeSuccess), rcode.Int())

	resolvedIP, ok := dp.Attributes().Get("dns.resolved.ip")
	require.True(t, ok)
	require.Equal(t, "93.184.216.34", resolvedIP.Str())

	resolvedAllIPs, ok := dp.Attributes().Get("dns.resolved.all.ips")
	require.True(t, ok)
	require.Equal(t, "93.184.216.34", resolvedAllIPs.Str())

	duration := findMetric(t, metrics, "dnscheck.duration")
	require.Equal(t, 1, duration.Gauge().DataPoints().Len())
}

// TestScrapeSuccessNonAddressRecordType verifies that dnscheck.status reports
// success for record types that don't resolve to an IP address (e.g. MX),
// since success must be driven by the response RCODE, not by the presence of
// an A/AAAA answer.
func TestScrapeSuccessNonAddressRecordType(t *testing.T) {
	addr := startTestDNSServer(t)

	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		DNSServers:           []DNSServerConfig{{Endpoint: addr, Timeout: 2 * time.Second}},
		Hostnames:            []HostnameConfig{{Name: "example.com", RecordType: "MX"}},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	status := findMetric(t, metrics, "dnscheck.status")
	dp := status.Sum().DataPoints().At(0)
	require.Equal(t, int64(1), dp.IntValue(), "MX query with a successful RCODE must report status success")

	rcode, ok := dp.Attributes().Get("dns.rcode")
	require.True(t, ok)
	require.Equal(t, int64(dns.RcodeSuccess), rcode.Int())

	_, ok = dp.Attributes().Get("dns.resolved.ip")
	require.False(t, ok, "dns.resolved.ip must be absent for non-address record types")
	_, ok = dp.Attributes().Get("dns.resolved.all.ips")
	require.False(t, ok, "dns.resolved.all.ips must be absent for non-address record types")
}

func TestScrapeTruncatedResponse(t *testing.T) {
	tests := []struct {
		name   string
		answer string
	}{
		{name: "empty answer"},
		{name: "partial answer", answer: "example.com. 60 IN A 192.0.2.42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc, err := net.ListenPacket("udp", "127.0.0.1:0")
			require.NoError(t, err)

			var answer dns.RR
			if tt.answer != "" {
				answer, err = dns.NewRR(tt.answer)
				require.NoError(t, err)
			}

			mux := dns.NewServeMux()
			mux.HandleFunc("example.com.", func(w dns.ResponseWriter, r *dns.Msg) {
				m := new(dns.Msg)
				m.SetReply(r)
				m.Truncated = true
				if answer != nil {
					m.Answer = append(m.Answer, answer)
				}
				_ = w.WriteMsg(m)
			})

			dnsServer := &dns.Server{PacketConn: pc, Handler: mux}
			started := make(chan struct{})
			dnsServer.NotifyStartedFunc = func() { close(started) }
			go func() { _ = dnsServer.ActivateAndServe() }()
			t.Cleanup(func() { _ = dnsServer.Shutdown() })
			<-started

			cfg := &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				DNSServers:           []DNSServerConfig{{Endpoint: pc.LocalAddr().String(), Timeout: 2 * time.Second}},
				Hostnames:            []HostnameConfig{{Name: "example.com", RecordType: "A"}},
			}
			s := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
			metrics, err := s.scrape(t.Context())
			require.NoError(t, err)

			dp := findMetric(t, metrics, "dnscheck.status").Sum().DataPoints().At(0)
			require.Equal(t, int64(0), dp.IntValue())

			rcode, ok := dp.Attributes().Get("dns.rcode")
			require.True(t, ok)
			require.Equal(t, int64(dns.RcodeSuccess), rcode.Int())

			_, hasIP := dp.Attributes().Get("dns.resolved.ip")
			_, hasIPs := dp.Attributes().Get("dns.resolved.all.ips")
			require.False(t, hasIP)
			require.False(t, hasIPs)
		})
	}
}

func TestScrapeNXDOMAIN(t *testing.T) {
	addr := startTestDNSServer(t)

	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		DNSServers:           []DNSServerConfig{{Endpoint: addr, Timeout: 2 * time.Second}},
		Hostnames:            []HostnameConfig{{Name: "nonexistent.example", RecordType: "A"}},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	status := findMetric(t, metrics, "dnscheck.status")
	dp := status.Sum().DataPoints().At(0)
	require.Equal(t, int64(0), dp.IntValue())

	rcode, ok := dp.Attributes().Get("dns.rcode")
	require.True(t, ok)
	require.Equal(t, int64(dns.RcodeNameError), rcode.Int())

	_, ok = dp.Attributes().Get("dns.resolved.ip")
	require.False(t, ok, "dns.resolved.ip must be absent on failed lookups")
	_, ok = dp.Attributes().Get("dns.resolved.all.ips")
	require.False(t, ok, "dns.resolved.all.ips must be absent on failed lookups")
}

func TestScrapeNoResponse(t *testing.T) {
	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		DNSServers:           []DNSServerConfig{{Endpoint: "127.0.0.1:1", Timeout: 200 * time.Millisecond}},
		Hostnames:            []HostnameConfig{{Name: "example.com", RecordType: "A"}},
	}
	cfg.MetricsBuilderConfig.Metrics.DnscheckError.Enabled = true
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)

	metrics, err := s.scrape(t.Context())
	require.Error(t, err)

	status := findMetric(t, metrics, "dnscheck.status")
	dp := status.Sum().DataPoints().At(0)
	require.Equal(t, int64(0), dp.IntValue())

	_, ok := dp.Attributes().Get("dns.rcode")
	require.False(t, ok, "dns.rcode must be absent when no response was received")

	errMetric := findMetric(t, metrics, "dnscheck.error")
	require.Equal(t, 1, errMetric.Sum().DataPoints().Len())
}

func TestScrapeResourceAttribution(t *testing.T) {
	addr := startTestDNSServer(t)

	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		DNSServers:           []DNSServerConfig{{Endpoint: addr, Timeout: 2 * time.Second}},
		Hostnames: []HostnameConfig{
			{Name: "example.com", RecordType: "A"},
			{Name: "nonexistent.example", RecordType: "A"},
		},
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	s := newScraper(cfg, settings)

	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)

	seenDomains := make(map[string]int64)
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		domain, ok := rm.Resource().Attributes().Get("dns.domain")
		require.True(t, ok, "resource must carry a dns.domain attribute")

		ms := rm.ScopeMetrics().At(0).Metrics()
		for j := 0; j < ms.Len(); j++ {
			if ms.At(j).Name() != "dnscheck.status" {
				continue
			}
			dp := ms.At(j).Sum().DataPoints().At(0)
			seenDomains[domain.Str()] = dp.IntValue()
		}
	}

	require.Equal(t, int64(1), seenDomains["example.com"], "example.com should report success")
	require.Equal(t, int64(0), seenDomains["nonexistent.example"], "nonexistent.example should report failure")
}

func findMetric(t *testing.T, metrics pmetric.Metrics, name string) pmetric.Metric {
	t.Helper()
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				if ms.At(k).Name() == name {
					return ms.At(k)
				}
			}
		}
	}
	t.Fatalf("metric %q not found", name)
	return pmetric.Metric{}
}
