// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver/internal/metadata"
)

var errMissingTargets = errors.New(`No targets specified`)

type scraper struct {
	cfg                *Config
	settings           receiver.Settings
	getConnectionState func(endpoint string) (tls.ConnectionState, error)
}

// listens to the error channel and combines errors sent from different go routines,
// returning the combined error list should context timeout or a nil error value is
// sent in the channel signifying the end of a scrape cycle
func errorListener(ctx context.Context, eQueue <-chan error, eOut chan<- *scrapererror.ScrapeErrors) {
	errs := &scrapererror.ScrapeErrors{}

	for {
		select {
		case <-ctx.Done():
			eOut <- errs
			return
		case err, ok := <-eQueue:
			if !ok {
				eOut <- errs
				return
			}
			errs.AddPartial(1, err)
		}
	}
}

func validatePort(port string) error {
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("provided port is not a number: %s", port)
	}
	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("provided port is out of valid range [1, 65535]: %d", portNum)
	}
	return nil
}

func validateEndpoint(endpoint string) error {
	if strings.Contains(endpoint, "://") {
		return fmt.Errorf("endpoint contains a scheme, which is not allowed: %s", endpoint)
	}
	_, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), err)
	}
	if err := validatePort(port); err != nil {
		return fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), err)
	}
	return nil
}

func validateFilepath(filePath string) error {
	cleanPath := filepath.Clean(filePath)
	fileInfo, err := os.Stat(cleanPath)
	if err != nil || !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("error accessing certificate file %s: %w", filePath, err)
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", cleanPath)
	}
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("certificate path is not a regular file: %s", filePath)
	}
	if _, err := os.ReadFile(cleanPath); err != nil {
		return fmt.Errorf("certificate file is not readable: %s", filePath)
	}
	return nil
}

func getConnectionState(endpoint string) (tls.ConnectionState, error) {
	conn, err := tls.Dial("tcp", endpoint, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return tls.ConnectionState{}, err
	}
	defer conn.Close()
	return conn.ConnectionState(), nil
}

func (s *scraper) scrapeEndpoint(endpoint string, metrics *pmetric.Metrics, wg *sync.WaitGroup, mux *sync.Mutex, errs chan error) {
	defer wg.Done()
	if err := validateEndpoint(endpoint); err != nil {
		s.settings.Logger.Error("Failed to validate endpoint", zap.String("endpoint", endpoint), zap.Error(err))
		errs <- err
		return
	}

	state, err := s.getConnectionState(endpoint)
	if err != nil {
		s.settings.Logger.Error("TCP connection error encountered", zap.String("endpoint", endpoint), zap.Error(err))
		errs <- err
		return
	}

	s.settings.Logger.Info("Peer Certificates", zap.Int("certificates_count", len(state.PeerCertificates)))
	if len(state.PeerCertificates) == 0 {
		err := fmt.Errorf("no TLS certificates found for endpoint: %s. Verify the endpoint serves TLS certificates", endpoint)
		s.settings.Logger.Error(err.Error(), zap.String("endpoint", endpoint))
		errs <- err
		return
	}

	cert := state.PeerCertificates[0]
	issuer := cert.Issuer.String()
	commonName := cert.Subject.CommonName
	sans := make([]any, len(cert.DNSNames)+len(cert.IPAddresses)+len(cert.URIs)+len(cert.EmailAddresses))
	i := 0
	for _, ip := range cert.IPAddresses {
		sans[i] = ip.String()
		i++
	}

	for _, uri := range cert.URIs {
		sans[i] = uri.String()
		i++
	}

	for _, dnsName := range cert.DNSNames {
		sans[i] = dnsName
		i++
	}

	for _, emailAddress := range cert.EmailAddresses {
		sans[i] = emailAddress
		i++
	}

	currentTime := time.Now()
	timeLeft := cert.NotAfter.Sub(currentTime).Seconds()
	timeLeftInt := int64(timeLeft)
	now := pcommon.NewTimestampFromTime(time.Now())

	mux.Lock()
	defer mux.Unlock()

	mb := metadata.NewMetricsBuilder(s.cfg.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.NewTimestampFromTime(time.Now())))
	rb := mb.NewResourceBuilder()
	rb.SetTlscheckTarget(endpoint)
	mb.RecordTlscheckTimeLeftDataPoint(now, timeLeftInt, issuer, commonName, sans)
	resourceMetrics := mb.Emit(metadata.WithResource(rb.Emit()))
	resourceMetrics.ResourceMetrics().At(0).MoveTo(metrics.ResourceMetrics().AppendEmpty())
}

func (s *scraper) scrapeFile(filePath string, metrics *pmetric.Metrics, wg *sync.WaitGroup, mux *sync.Mutex, errs chan error) {
	defer wg.Done()
	if err := validateFilepath(filePath); err != nil {
		s.settings.Logger.Error("Failed to validate certificate file", zap.String("file_path", filePath), zap.Error(err))
		errs <- err
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		s.settings.Logger.Error("Failed to open certificate file", zap.String("file_path", filePath), zap.Error(err))
		errs <- err
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		s.settings.Logger.Error("Failed to read certificate file", zap.String("file_path", filePath), zap.Error(err))
		errs <- err
		return
	}

	var certs []*x509.Certificate
	remaining := data

	for len(remaining) > 0 {
		var block *pem.Block
		block, remaining = pem.Decode(remaining)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			continue
		}

		cert, parseErr := x509.ParseCertificate(block.Bytes)
		if parseErr != nil {
			s.settings.Logger.Error("Failed to parse certificate", zap.String("file_path", filePath), zap.Error(parseErr))
			errs <- parseErr
			return
		}
		certs = append(certs, cert)
	}

	if len(certs) == 0 {
		s.settings.Logger.Error("No valid certificates found in file", zap.String("file_path", filePath))
		errs <- err
		return
	}

	s.settings.Logger.Debug("Found certificates in chain", zap.String("file_path", filePath), zap.Int("count", len(certs)))

	cert := certs[0] // Use the leaf certificate
	issuer := cert.Issuer.String()
	commonName := cert.Subject.CommonName
	sans := make([]any, len(cert.DNSNames)+len(cert.IPAddresses)+len(cert.URIs)+len(cert.EmailAddresses))
	i := 0
	for _, ip := range cert.IPAddresses {
		sans[i] = ip.String()
		i++
	}

	for _, uri := range cert.URIs {
		sans[i] = uri.String()
		i++
	}

	for _, dnsName := range cert.DNSNames {
		sans[i] = dnsName
		i++
	}

	for _, emailAddress := range cert.EmailAddresses {
		sans[i] = emailAddress
		i++
	}

	currentTime := time.Now()
	timeLeft := cert.NotAfter.Sub(currentTime).Seconds()
	timeLeftInt := int64(timeLeft)
	now := pcommon.NewTimestampFromTime(time.Now())

	mux.Lock()
	defer mux.Unlock()

	mb := metadata.NewMetricsBuilder(s.cfg.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.NewTimestampFromTime(time.Now())))
	rb := mb.NewResourceBuilder()
	rb.SetTlscheckTarget(filePath)
	mb.RecordTlscheckTimeLeftDataPoint(now, timeLeftInt, issuer, commonName, sans)
	resourceMetrics := mb.Emit(metadata.WithResource(rb.Emit()))
	resourceMetrics.ResourceMetrics().At(0).MoveTo(metrics.ResourceMetrics().AppendEmpty())
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var errs *scrapererror.ScrapeErrors

	errOut := make(chan *scrapererror.ScrapeErrors)
	errChan := make(chan error, len(s.cfg.Targets))
	go func() {
		errorListener(ctx, errChan, errOut)
	}()

	if s.cfg == nil || len(s.cfg.Targets) == 0 {
		return pmetric.NewMetrics(), errMissingTargets
	}

	var wg sync.WaitGroup
	wg.Add(len(s.cfg.Targets))
	var mux sync.Mutex

	metrics := pmetric.NewMetrics()

	for _, target := range s.cfg.Targets {
		if target.FilePath != "" {
			go s.scrapeFile(target.FilePath, &metrics, &wg, &mux, errChan)
		} else {
			go s.scrapeEndpoint(target.Endpoint, &metrics, &wg, &mux, errChan)
		}
	}

	wg.Wait()
	close(errChan)
	errs = <-errOut
	return metrics, errs.Combine()
}

func newScraper(cfg *Config, settings receiver.Settings, getConnectionState func(endpoint string) (tls.ConnectionState, error)) *scraper {
	return &scraper{
		cfg:                cfg,
		settings:           settings,
		getConnectionState: getConnectionState,
	}
}
