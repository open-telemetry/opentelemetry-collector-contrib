// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"context"
	"go.opentelemetry.io/collector/config/confignet"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

type scraper struct {
	cfg      *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
	// dial sth
	getConnectionState func(tcpConfig *confignet.TCPAddrConfig) (TCPConnectionState, error)
}

type TCPConnectionState struct {
	LocalAddr  string // Local address of the connection
	RemoteAddr string // Remote address of the connection
	Network    string // Network type (e.g., "tcp")
}

// we may not need
func getConnectionState(tcpConfig *confignet.TCPAddrConfig) (TCPConnectionState, error) {

	conn, err := tcpConfig.Dial(context.Background())
	//conn, err := tls.Dial("tcp", endpoint, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return TCPConnectionState{}, err
	}
	defer conn.Close()
	//state := TCPConnectionState{
	//	LocalAddr:  conn.LocalAddr().String(),  // Local endpoint (IP:port)
	//	RemoteAddr: conn.RemoteAddr().String(), // Remote endpoint (IP:port)
	//	Network:    conn.LocalAddr().Network(), // Connection network type
	//}
	return TCPConnectionState{}, nil
}

func (s *scraper) scrapeEndpoint(tcpConfig *confignet.TCPAddrConfig, wg *sync.WaitGroup, mux *sync.Mutex) {
	defer wg.Done()
	const pointVal int64 = 1 // Use a constant for clarity and immutability
	start := time.Now()
	// Attempt to get the connection state
	//tcpClient, err := tcpConfig.ToClient()
	_, err1 := s.getConnectionState(tcpConfig)
	now := pcommon.NewTimestampFromTime(time.Now())

	if err1 != nil {
		// Record error data point and log the error
		mux.Lock()
		s.mb.RecordTcpcheckErrorDataPoint(now, pointVal, tcpConfig.Endpoint, err1.Error())
		s.settings.Logger.Error("TCP connection error encountered", zap.String("endpoint", tcpConfig.Endpoint), zap.Error(err1))
		defer mux.Unlock()
		return
	}

	// Record success metrics
	duration := time.Since(start).Milliseconds()

	mux.Lock()
	s.mb.RecordTcpcheckDurationDataPoint(now, duration, tcpConfig.Endpoint)
	s.mb.RecordTcpcheckStatusDataPoint(now, pointVal, tcpConfig.Endpoint)
	defer mux.Unlock()
}

//func (s *scraper) scrapeEndpoint(tcpConfig *confignet.TCPAddrConfig, wg *sync.WaitGroup, mux *sync.Mutex) {
//	defer wg.Done()
//
//	//state, err := s.getConnectionState(endpoint)
//	//if err != nil {
//	//	s.settings.Logger.Error("TCP connection error encountered", zap.String("endpoint", endpoint), zap.Error(err))
//	//	return
//	//}
//
//	currentTime := time.Now()
//	timeLeftInt := int64(timeLeft)
//	now := pcommon.NewTimestampFromTime(time.Now())
//
//	mux.Lock()
//	defer mux.Unlock()
//	s.mb.RecordTlscheckTimeLeftDataPoint(now, timeLeftInt, issuer, commonName, endpoint)
//}

//# scrape -> to client -> dial

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	if s.cfg == nil || len(s.cfg.tcpConfigs) == 0 {
		return pmetric.NewMetrics(), errMissingTargets
	}

	var wg sync.WaitGroup
	wg.Add(len(s.cfg.tcpConfigs))
	var mux sync.Mutex

	for _, tcpConfig := range s.cfg.tcpConfigs {
		go s.scrapeEndpoint(tcpConfig, &wg, &mux)
	}

	wg.Wait()
	return s.mb.Emit(), nil
}

func newScraper(cfg *Config, settings receiver.Settings) *scraper {
	return &scraper{
		cfg:      cfg,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), settings),
		//getConnectionState: getConnectionState,
	}
}

// others
//var errClientNotInit = errors.New("client not initialized")
//
//type tcpCheckScraper struct {
//	*configtcp.Client
//	*Config
//	settings component.TelemetrySettings
//	mb       *metadata.MetricsBuilder
//}
//
//// start the scraper by creating a new TCP Client on the scraper
//func (c *tcpCheckScraper) start(_ context.Context, host component.Host) error {
//	var err error
//	c.Client, err = c.Config.ToClient(host, c.settings)
//	return err
//}
//
//func (c *tcpCheckScraper) start() error {
//	c.Client = make([]*Client, len(c.Config.TCPClientSettings)) // Allocate slice for clients
//
//	// Loop through the array of TCPClientSettings and call ToClient
//	for i, tcpSettings := range c.Config.TCPClientSettings {
//		client, clientErr := tcpSettings.ToClient()
//		if clientErr != nil {
//			return fmt.Errorf("failed to create client for index %d: %w", i, clientErr)
//		}
//		c.Client[i] = client // Store the client
//	}
//
//	return nil
//}
//
//func (c *tcpCheckScraper) scrapeTCP(now pcommon.Timestamp) error {
//	var success int64
//
//	start := time.Now()
//	err := c.Client.Dial()
//	if err == nil {
//		success = 1
//	}
//	c.mb.RecordTcpcheckDurationDataPoint(now, time.Since(start).Nanoseconds(), c.Config.TCPClientSettings.Endpoint)
//	c.mb.RecordTcpcheckStatusDataPoint(now, success, c.Config.TCPClientSettings.Endpoint)
//	return err
//}
//
//// timeout chooses the shorter between a given deadline and timeout
//func timeout(deadline time.Time, timeout time.Duration) time.Duration {
//	timeToDeadline := time.Until(deadline)
//	if timeToDeadline < timeout {
//		return timeToDeadline
//	}
//	return timeout
//}
//
//// scrape connects to the endpoint and produces metrics based on the response.
//func (c *tcpCheckScraper) scrape(ctx context.Context) (_ pmetric.Metrics, err error) {
//	var (
//		to time.Duration
//	)
//	// check cancellation
//	select {
//	case <-ctx.Done():
//		return pmetric.NewMetrics(), ctx.Err()
//	default:
//	}
//
//	cleanup := func() {
//		c.Client.Close()
//	}
//
//	// if the context carries a shorter deadline then timeout that quickly
//	deadline, ok := ctx.Deadline()
//	if ok {
//		to = timeout(deadline, c.Client.TCPAddrConfig.DialerConfig.Timeout)
//		c.Client.TCPAddrConfig.DialerConfig.Timeout = to
//	}
//
//	ctx, cancel := context.WithCancel(ctx)
//	defer cancel()
//
//	now := pcommon.NewTimestampFromTime(time.Now())
//	if c.Client == nil {
//		return pmetric.NewMetrics(), errClientNotInit
//	}
//
//	if err = c.scrapeTCP(now); err != nil {
//		c.mb.RecordTcpcheckErrorDataPoint(now, int64(1), c.Endpoint, err.Error())
//	} else {
//		go func() {
//			<-ctx.Done()
//			cleanup()
//		}()
//	}
//
//	return c.mb.Emit(), nil
//}
//
//func newScraper(conf *Config, settings receiver.Settings) *tcpCheckScraper {
//	return &tcpCheckScraper{
//		Config:   conf,
//		settings: settings.TelemetrySettings,
//		mb:       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
//	}
//}
