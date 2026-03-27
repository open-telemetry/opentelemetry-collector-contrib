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

// certResult holds parsed certificate data returned by a worker.
type certResult struct {
	target string
	cert   *x509.Certificate
}

type scrapeResult struct {
	certs []certResult
	err   error
}

type resourceGroup struct {
	target  string
	results []certResult
}

type scraper struct {
	cfg                *Config
	settings           receiver.Settings
	getConnectionState func(endpoint string) (tls.ConnectionState, error)
	mb                 *metadata.MetricsBuilder
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

func certSANs(cert *x509.Certificate) []any {
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
	return sans
}

func (s *scraper) resourceGroupKey(target string) string {
	if s.cfg.ResourceAttributes.TlscheckTarget.Enabled {
		return target
	}
	return ""
}

func (s *scraper) groupByResource(results []certResult) []resourceGroup {
	groups := make([]resourceGroup, 0, len(results))
	groupIndex := make(map[string]int, len(results))

	for _, result := range results {
		key := s.resourceGroupKey(result.target)
		if idx, ok := groupIndex[key]; ok {
			groups[idx].results = append(groups[idx].results, result)
			continue
		}

		groupIndex[key] = len(groups)
		groups = append(groups, resourceGroup{
			target:  result.target,
			results: []certResult{result},
		})
	}

	return groups
}

func (s *scraper) recordCertMetrics(now time.Time, ts pcommon.Timestamp, cert *x509.Certificate) {
	s.mb.RecordTlscheckTimeLeftDataPoint(
		ts,
		int64(cert.NotAfter.Sub(now).Seconds()),
		cert.Issuer.String(),
		cert.Subject.CommonName,
		certSANs(cert),
	)
}

func (s *scraper) scrapeEndpoint(endpoint string) ([]certResult, error) {
	if err := validateEndpoint(endpoint); err != nil {
		s.settings.Logger.Error("Failed to validate endpoint", zap.String("endpoint", endpoint), zap.Error(err))
		return nil, err
	}

	state, err := s.getConnectionState(endpoint)
	if err != nil {
		s.settings.Logger.Error("TCP connection error encountered", zap.String("endpoint", endpoint), zap.Error(err))
		return nil, err
	}

	s.settings.Logger.Debug("Peer Certificates", zap.Int("certificates_count", len(state.PeerCertificates)))
	if len(state.PeerCertificates) == 0 {
		err := fmt.Errorf("no TLS certificates found for endpoint: %s. Verify the endpoint serves TLS certificates", endpoint)
		s.settings.Logger.Error(err.Error(), zap.String("endpoint", endpoint))
		return nil, err
	}

	return []certResult{{target: endpoint, cert: state.PeerCertificates[0]}}, nil
}

func (s *scraper) scrapeFile(target *CertificateTarget) ([]certResult, error) {
	filePath := target.FilePath

	if err := validateFilepath(filePath); err != nil {
		s.settings.Logger.Error("Failed to validate certificate file", zap.String("file_path", filePath), zap.Error(err))
		return nil, err
	}

	format := resolveFileFormat(target.FileFormat, filePath)

	switch format {
	case FileFormatJKS:
		return s.scrapeJKS(target)
	case FileFormatPKCS12:
		return s.scrapePKCS12(target)
	default:
		return s.scrapePEM(filePath)
	}
}

func (s *scraper) scrapePEM(filePath string) ([]certResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		s.settings.Logger.Error("Failed to open certificate file", zap.String("file_path", filePath), zap.Error(err))
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		s.settings.Logger.Error("Failed to read certificate file", zap.String("file_path", filePath), zap.Error(err))
		return nil, err
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
			return nil, parseErr
		}
		certs = append(certs, cert)
	}

	if len(certs) == 0 {
		err := fmt.Errorf("no valid certificates found in PEM file: %s", filePath)
		s.settings.Logger.Error(err.Error(), zap.String("file_path", filePath))
		return nil, err
	}

	s.settings.Logger.Debug("Found certificates in chain", zap.String("file_path", filePath), zap.Int("count", len(certs)))

	// Use the leaf certificate (first in file)
	return []certResult{{target: filePath, cert: certs[0]}}, nil
}

func (s *scraper) scrapeJKS(target *CertificateTarget) ([]certResult, error) {
	filePath := target.FilePath
	password := []byte(string(target.Password))

	file, err := os.Open(filePath)
	if err != nil {
		s.settings.Logger.Error("Failed to open JKS file", zap.String("file_path", filePath), zap.Error(err))
		return nil, err
	}
	defer file.Close()

	ks := keystore.New()
	if err := ks.Load(file, password); err != nil {
		s.settings.Logger.Error("Failed to load JKS keystore", zap.String("file_path", filePath), zap.Error(err))
		return nil, err
	}

	aliases := ks.Aliases()
	if len(aliases) == 0 {
		err := fmt.Errorf("no entries found in JKS keystore: %s", filePath)
		s.settings.Logger.Error(err.Error(), zap.String("file_path", filePath))
		return nil, err
	}

	results := make([]certResult, 0, len(aliases))
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
			results = append(results, certResult{target: filePath, cert: cert})
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
			results = append(results, certResult{target: filePath, cert: cert})
		} else {
			lastAliasErr = err
			s.settings.Logger.Debug("Failed to read alias from JKS keystore",
				zap.String("file_path", filePath),
				zap.String("alias", alias),
				zap.Error(err))
		}
	}

	if len(results) == 0 {
		err := fmt.Errorf("no parseable certificates found in JKS keystore: %s", filePath)
		if lastAliasErr != nil {
			err = fmt.Errorf("%w: last alias error: %w", err, lastAliasErr)
		}
		s.settings.Logger.Error(err.Error(), zap.String("file_path", filePath))
		return nil, err
	}

	return results, nil
}

func (s *scraper) scrapePKCS12(target *CertificateTarget) ([]certResult, error) {
	filePath := target.FilePath
	password := string(target.Password)

	data, err := os.ReadFile(filePath)
	if err != nil {
		s.settings.Logger.Error("Failed to read PKCS#12 file", zap.String("file_path", filePath), zap.Error(err))
		return nil, err
	}

	_, cert, _, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		s.settings.Logger.Error("Failed to decode PKCS#12 keystore", zap.String("file_path", filePath), zap.Error(err))
		return nil, err
	}

	if cert == nil {
		err := fmt.Errorf("no leaf certificate found in PKCS#12 file: %s", filePath)
		s.settings.Logger.Error(err.Error(), zap.String("file_path", filePath))
		return nil, err
	}

	return []certResult{{target: filePath, cert: cert}}, nil
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if s.cfg == nil || len(s.cfg.Targets) == 0 {
		return pmetric.NewMetrics(), errMissingTargets
	}
	if err := ctx.Err(); err != nil {
		return pmetric.NewMetrics(), err
	}

	var wg sync.WaitGroup
	wg.Add(len(s.cfg.Targets))
	resultsCh := make(chan scrapeResult, len(s.cfg.Targets))

	for _, target := range s.cfg.Targets {
		go func(target *CertificateTarget) {
			defer wg.Done()

			var (
				certs []certResult
				err   error
			)
			if target.FilePath != "" {
				certs, err = s.scrapeFile(target)
			} else {
				certs, err = s.scrapeEndpoint(target.Endpoint)
			}

			resultsCh <- scrapeResult{certs: certs, err: err}
		}(target)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	errs := &scrapererror.ScrapeErrors{}
	results := make([]certResult, 0, len(s.cfg.Targets))
	for result := range resultsCh {
		results = append(results, result.certs...)
		if result.err != nil {
			errs.AddPartial(1, result.err)
		}
	}
	if err := ctx.Err(); err != nil {
		errs.AddPartial(1, err)
	}

	if len(results) == 0 {
		return pmetric.NewMetrics(), errs.Combine()
	}

	now := time.Now()
	ts := pcommon.NewTimestampFromTime(now)
	s.mb.Reset(metadata.WithStartTime(ts))

	for _, group := range s.groupByResource(results) {
		rb := s.mb.NewResourceBuilder()
		rb.SetTlscheckTarget(group.target)

		for _, result := range group.results {
			s.recordCertMetrics(now, ts, result.cert)
		}

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}

	metrics := s.mb.Emit()
	return metrics, errs.Combine()
}

func newScraper(cfg *Config, settings receiver.Settings, getConnectionState func(endpoint string) (tls.ConnectionState, error)) *scraper {
	var mb *metadata.MetricsBuilder
	if cfg != nil {
		mb = metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)
	}

	return &scraper{
		cfg:                cfg,
		settings:           settings,
		getConnectionState: getConnectionState,
		mb:                 mb,
	}
}
