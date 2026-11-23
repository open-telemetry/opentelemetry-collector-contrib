// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	kustov2 "github.com/Azure/azure-kusto-go/azkustoingestv2"
	v2options "github.com/Azure/azure-kusto-go/azkustoingestv2/ingestoptions"
	"go.uber.org/zap"
)

type ingestionConfigurationClient interface {
	GetConfigurationDetails(context.Context) (*v2options.ConfigurationResponse, error)
}

type queuedV2Client interface {
	Ingest(context.Context, string, string, v2options.IngestionSource, *v2options.IngestRequestProperties) (*v2options.ExtendedIngestResponse, error)
	Close() error
}

type ingestV2State struct {
	mu              sync.Mutex
	configuration   ingestionConfigurationClient
	newClient       func() (queuedV2Client, error)
	client          queuedV2Client
	lastConfig      *v2options.ConfigurationResponse
	refreshAt       time.Time
	now             func() time.Time
	retryBackoff    time.Duration
	database, table string
	mapping         string
	logger          *zap.Logger
	closed          bool
}

func newIngestV2State(cfg *Config, version, table string, dataType int, logger *zap.Logger) (*ingestV2State, error) {
	dmURL, err := ingestV2DMURL(cfg.ClusterURI)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{Timeout: 30 * time.Second}
	auth := &ingestV2Authentication{config: *cfg, dmURL: dmURL, httpClient: httpClient, logger: logger}
	opts := []kustov2.ClientOption{
		kustov2.WithHTTPClient(httpClient),
		kustov2.WithTokenProvider(auth.token),
		kustov2.WithUploadTokenCredential(auth),
		kustov2.WithClientDetails(v2options.NewClientDetails("OpenTelemetry", "", version)),
	}
	configuration, err := kustov2.NewConfigurationClient(dmURL, opts...)
	if err != nil {
		return nil, err
	}
	state := &ingestV2State{
		configuration: configuration,
		newClient:     func() (queuedV2Client, error) { return kustov2.NewQueuedClient(dmURL, opts...) },
		now:           time.Now, database: cfg.Database, table: table, logger: logger,
		retryBackoff: 2 * time.Second,
	}
	switch dataType {
	case metricsType:
		state.mapping = cfg.MetricTableMapping
	case logsType:
		state.mapping = cfg.LogTableMapping
	case tracesType:
		state.mapping = cfg.TraceTableMapping
	}
	return state, nil
}

func (s *ingestV2State) selectIngestion(ctx context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if s.closed {
		return false, fmt.Errorf("ingest V2 exporter is closed")
	}
	if !s.now().Before(s.refreshAt) {
		configuration, err := s.loadConfiguration(ctx)
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if err != nil || configuration == nil {
			if s.lastConfig != nil {
				s.refreshAt = s.now().Add(time.Minute)
				s.logger.Warn("Could not refresh DM ingestion configuration; retaining the last configuration")
			} else {
				s.refreshAt = s.now().Add(10 * time.Second)
				s.logger.Warn("Could not read DM ingestion configuration; using legacy ingestion")
			}
		} else {
			s.lastConfig = configuration
			s.refreshAt = s.now().Add(ingestV2RefreshInterval(configuration))
		}
	}
	if !ingestV2Preferred(s.lastConfig) {
		return false, nil
	}
	if s.client == nil {
		client, err := s.newClient()
		if err != nil {
			s.logger.Warn("Could not create the V2 queued client; using legacy ingestion", zap.Error(err))
			return false, nil
		}
		s.client = client
		s.logger.Info("Using V2 queued ingestion")
	}
	return true, nil
}

func (s *ingestV2State) loadConfiguration(ctx context.Context) (*v2options.ConfigurationResponse, error) {
	attempts := 1
	if s.lastConfig == nil {
		attempts = 3
	}
	for attempt := 0; ; attempt++ {
		configuration, err := s.configuration.GetConfigurationDetails(ctx)
		if err == nil && configuration != nil {
			return configuration, nil
		}
		if err == nil {
			err = errors.New("DM ingestion configuration is empty")
		}
		var networkErr net.Error
		if attempt+1 == attempts || ctx.Err() != nil || errors.As(err, &networkErr) && networkErr.Timeout() {
			return nil, err
		}
		timer := time.NewTimer(s.retryBackoff * time.Duration(1<<attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (s *ingestV2State) ingest(ctx context.Context, reader io.ReadCloser) error {
	source := v2options.NewStreamSource(reader, v2options.FormatJSON)
	_, err := s.client.Ingest(ctx, s.database, s.table, source, &v2options.IngestRequestProperties{
		Format: v2options.FormatJSON, IngestionMappingRef: s.mapping,
	})
	if err != nil {
		s.logger.Error("Error performing V2 queued ingestion", zap.Error(err))
	}
	return err
}

func (s *ingestV2State) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func ingestV2Preferred(cfg *v2options.ConfigurationResponse) bool {
	return cfg != nil && ((cfg.IngestionSettings != nil &&
		strings.EqualFold(strings.TrimSpace(cfg.IngestionSettings.PreferredIngestionMethod), "Rest")) ||
		(cfg.ContainerSettings != nil &&
			strings.EqualFold(strings.TrimSpace(cfg.ContainerSettings.PreferredUploadMethod), "Lake")))
}

func ingestV2RefreshInterval(cfg *v2options.ConfigurationResponse) time.Duration {
	const maxInterval = time.Hour
	if cfg.ContainerSettings == nil {
		return maxInterval
	}
	value := strings.TrimSpace(cfg.ContainerSettings.RefreshInterval)
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return maxInterval
	}
	hours := strings.Split(parts[0], ".")
	var days int64
	if len(hours) == 2 {
		var err error
		days, err = strconv.ParseInt(hours[0], 10, 64)
		if err != nil || days < 0 {
			return maxInterval
		}
	} else if len(hours) != 1 {
		return maxInterval
	}
	h, errH := strconv.ParseInt(hours[len(hours)-1], 10, 64)
	m, errM := strconv.ParseInt(parts[1], 10, 64)
	seconds, errS := strconv.ParseFloat(parts[2], 64)
	if errH != nil || errM != nil || errS != nil || days != 0 || h < 0 || h >= 1 ||
		m < 0 || m >= 60 || seconds < 0 || seconds >= 60 {
		return maxInterval
	}
	interval := time.Duration(m)*time.Minute + time.Duration(seconds*float64(time.Second))
	if interval <= 0 {
		return maxInterval
	}
	return interval
}

func ingestV2DMURL(cluster string) (string, error) {
	u, err := url.Parse(cluster)
	if err != nil || u.Scheme != "https" && u.Scheme != "http" || u.Hostname() == "" {
		return "", fmt.Errorf("invalid Kusto cluster URI")
	}
	host := u.Hostname()
	lowerHost := strings.ToLower(host)
	if net.ParseIP(host) == nil && lowerHost != "localhost" && !strings.HasSuffix(lowerHost, ".localhost") &&
		lowerHost != "onebox.dev.kusto.windows.net" && !strings.HasPrefix(lowerHost, "ingest-") {
		host = "ingest-" + host
		if u.Port() != "" {
			host = net.JoinHostPort(host, u.Port())
		}
		u.Host = host
	}
	u.Path, u.RawPath, u.RawQuery, u.Fragment = "", "", "", ""
	return u.String(), nil
}
