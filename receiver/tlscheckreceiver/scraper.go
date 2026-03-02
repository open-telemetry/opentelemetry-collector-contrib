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

	keystore "github.com/pavlo-v-chernykh/keystore-go/v4"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"
	"software.sslmate.com/src/go-pkcs12"

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

// resolveFileFormat returns the effective FileFormat for a target.
// If format is auto (or the zero value ""), it infers from the file extension.
func resolveFileFormat(format FileFormat, filePath string) FileFormat {
	if format != FileFormatAuto && format != "" {
		return format
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".jks":
		return FileFormatJKS
	case ".p12", ".pfx":
		return FileFormatPKCS12
	default:
		return FileFormatPEM
	}
}

func getConnectionState(endpoint string) (tls.ConnectionState, error) {
	conn, err := tls.Dial("tcp", endpoint, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return tls.ConnectionState{}, err
	}
	defer conn.Close()
	return conn.ConnectionState(), nil
}

// extractCertMetrics records the tlscheck.time_left metric for a single leaf certificate.
// target is the resource label (endpoint string or file path).
func (s *scraper) extractCertMetrics(target string, cert *x509.Certificate, metrics *pmetric.Metrics, mux *sync.Mutex) {
	issuer := cert.Issuer.String()
	commonName := cert.Subject.CommonName

	sans := make([]any, 0, len(cert.DNSNames)+len(cert.IPAddresses)+len(cert.URIs)+len(cert.EmailAddresses))
	for _, ip := range cert.IPAddresses {
		sans = append(sans, ip.String())
	}
	for _, uri := range cert.URIs {
		sans = append(sans, uri.String())
	}
	for _, dnsName := range cert.DNSNames {
		sans = append(sans, dnsName)
	}
	for _, emailAddress := range cert.EmailAddresses {
		sans = append(sans, emailAddress)
	}

	currentTime := time.Now()
	timeLeft := cert.NotAfter.Sub(currentTime).Seconds()
	timeLeftInt := int64(timeLeft)
	now := pcommon.NewTimestampFromTime(currentTime)

	// Build per-certificate metrics outside the critical section to reduce lock contention.
	mb := metadata.NewMetricsBuilder(s.cfg.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.NewTimestampFromTime(currentTime)))
	rb := mb.NewResourceBuilder()
	rb.SetTlscheckTarget(target)
	mb.RecordTlscheckTimeLeftDataPoint(now, timeLeftInt, issuer, commonName, sans)
	resourceMetrics := mb.Emit(metadata.WithResource(rb.Emit()))

	// Only guard the mutation of the shared metrics object with the mutex.
	mux.Lock()
	resourceMetrics.ResourceMetrics().At(0).MoveTo(metrics.ResourceMetrics().AppendEmpty())
	mux.Unlock()
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

	s.settings.Logger.Debug("Peer Certificates", zap.Int("certificates_count", len(state.PeerCertificates)))
	if len(state.PeerCertificates) == 0 {
		err := fmt.Errorf("no TLS certificates found for endpoint: %s. Verify the endpoint serves TLS certificates", endpoint)
		s.settings.Logger.Error(err.Error(), zap.String("endpoint", endpoint))
		errs <- err
		return
	}

	s.extractCertMetrics(endpoint, state.PeerCertificates[0], metrics, mux)
}

func (s *scraper) scrapeFile(target *CertificateTarget, metrics *pmetric.Metrics, wg *sync.WaitGroup, mux *sync.Mutex, errs chan error) {
	defer wg.Done()
	filePath := target.FilePath

	if err := validateFilepath(filePath); err != nil {
		s.settings.Logger.Error("Failed to validate certificate file", zap.String("file_path", filePath), zap.Error(err))
		errs <- err
		return
	}

	format := resolveFileFormat(target.FileFormat, filePath)

	switch format {
	case FileFormatJKS:
		s.scrapeJKS(target, metrics, mux, errs)
	case FileFormatPKCS12:
		s.scrapePKCS12(target, metrics, mux, errs)
	default:
		s.scrapePEM(filePath, metrics, mux, errs)
	}
}

func (s *scraper) scrapePEM(filePath string, metrics *pmetric.Metrics, mux *sync.Mutex, errs chan error) {
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
		err := fmt.Errorf("no valid certificates found in PEM file: %s", filePath)
		s.settings.Logger.Error(err.Error(), zap.String("file_path", filePath))
		errs <- err
		return
	}

	s.settings.Logger.Debug("Found certificates in chain", zap.String("file_path", filePath), zap.Int("count", len(certs)))

	// Use the leaf certificate (first in file)
	s.extractCertMetrics(filePath, certs[0], metrics, mux)
}

func (s *scraper) scrapeJKS(target *CertificateTarget, metrics *pmetric.Metrics, mux *sync.Mutex, errs chan error) {
	filePath := target.FilePath
	password := []byte(string(target.Password))

	file, err := os.Open(filePath)
	if err != nil {
		s.settings.Logger.Error("Failed to open JKS file", zap.String("file_path", filePath), zap.Error(err))
		errs <- err
		return
	}
	defer file.Close()

	ks := keystore.New()
	if err := ks.Load(file, password); err != nil {
		s.settings.Logger.Error("Failed to load JKS keystore", zap.String("file_path", filePath), zap.Error(err))
		errs <- err
		return
	}

	aliases := ks.Aliases()
	if len(aliases) == 0 {
		err := fmt.Errorf("no entries found in JKS keystore: %s", filePath)
		s.settings.Logger.Error(err.Error(), zap.String("file_path", filePath))
		errs <- err
		return
	}

	foundAny := false
	var lastAliasErr error
	for _, alias := range aliases {
		if entry, err := ks.GetTrustedCertificateEntry(alias); err == nil {
			cert, parseErr := x509.ParseCertificate(entry.Certificate.Content)
			if parseErr != nil {
				s.settings.Logger.Warn("Failed to parse trusted certificate in JKS",
					zap.String("file_path", filePath),
					zap.String("alias", alias),
					zap.Error(parseErr))
				continue
			}
			s.extractCertMetrics(filePath, cert, metrics, mux)
			foundAny = true
		} else if entry, err := ks.GetPrivateKeyEntry(alias, password); err == nil {
			if len(entry.CertificateChain) == 0 {
				continue
			}
			// First cert in the chain is the leaf
			cert, parseErr := x509.ParseCertificate(entry.CertificateChain[0].Content)
			if parseErr != nil {
				s.settings.Logger.Warn("Failed to parse private key entry certificate in JKS",
					zap.String("file_path", filePath),
					zap.String("alias", alias),
					zap.Error(parseErr))
				continue
			}
			s.extractCertMetrics(filePath, cert, metrics, mux)
			foundAny = true
		} else {
			lastAliasErr = err
			s.settings.Logger.Debug("Failed to read alias from JKS keystore",
				zap.String("file_path", filePath),
				zap.String("alias", alias),
				zap.Error(err))
		}
	}

	if !foundAny {
		err := fmt.Errorf("no parseable certificates found in JKS keystore: %s", filePath)
		if lastAliasErr != nil {
			err = fmt.Errorf("%w: last alias error: %w", err, lastAliasErr)
		}
		s.settings.Logger.Error(err.Error(), zap.String("file_path", filePath))
		errs <- err
	}
}

func (s *scraper) scrapePKCS12(target *CertificateTarget, metrics *pmetric.Metrics, mux *sync.Mutex, errs chan error) {
	filePath := target.FilePath
	password := string(target.Password)

	data, err := os.ReadFile(filePath)
	if err != nil {
		s.settings.Logger.Error("Failed to read PKCS#12 file", zap.String("file_path", filePath), zap.Error(err))
		errs <- err
		return
	}

	_, cert, _, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		s.settings.Logger.Error("Failed to decode PKCS#12 keystore", zap.String("file_path", filePath), zap.Error(err))
		errs <- err
		return
	}

	if cert == nil {
		err := fmt.Errorf("no leaf certificate found in PKCS#12 file: %s", filePath)
		s.settings.Logger.Error(err.Error(), zap.String("file_path", filePath))
		errs <- err
		return
	}

	s.extractCertMetrics(filePath, cert, metrics, mux)
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
			go s.scrapeFile(target, &metrics, &wg, &mux, errChan)
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
