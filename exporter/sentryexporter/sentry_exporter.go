// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	internalrl "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter/internal/ratelimit"
)

var (
	errExporterShuttingDown = errors.New("sentry exporter is shutting down")
	errProjectBeingCreated  = errors.New("project is being created asynchronously, some events might be dropped")
)

const (
	maxProjects              = 1000
	projectCreationQueueSize = 1000
	timeout                  = 5 * time.Second
)

// newHTTPClient is used to override the http client factory. While running tests
// we need to mock the http transport, so that we don't open real sockets.
var newHTTPClient = func(ctx context.Context, cfg confighttp.ClientConfig, host component.Host, ts component.TelemetrySettings) (*http.Client, error) {
	var extensions map[component.ID]component.Component
	if host != nil {
		extensions = host.GetExtensions()
	}
	return cfg.ToClient(ctx, extensions, ts)
}

// projectCreationRequest represents a request to create a project
type projectCreationRequest struct {
	projectSlug string
	platform    string
}

// endpointState holds shared mutable state (client, caches) across signal exporters.
type endpointState struct {
	config            *Config
	telemetrySettings component.TelemetrySettings

	client       *http.Client
	sentryClient sentryAPIClient

	projectToEndpoint map[string]*otlpEndpoints
	projectMapMu      sync.RWMutex

	attributeKey    string
	projectMapping  map[string]string
	defaultTeamSlug string

	inflight singleflight.Group

	projectCreationQueue chan projectCreationRequest
	pendingCreations     sync.Map
	workerCtx            context.Context
	workerCancel         context.CancelFunc
	workerWg             sync.WaitGroup

	rateLimiter *rateLimiter

	startOnce    sync.Once
	startErr     error
	shutdownOnce sync.Once
	closing      atomic.Bool
}

// projectRoutingKey is a composite key for routing traces and logs by project slug and platform.
type projectRoutingKey struct {
	slug     string
	platform string
}

// sentryExporter is a per-signal wrapper that forwards to the shared endpointState.
type sentryExporter struct {
	logger *zap.Logger
	state  *endpointState
}

func newEndpointState(config *Config, set exporter.Settings) (*endpointState, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	workerCtx, workerCancel := context.WithCancel(context.Background())

	state := &endpointState{
		config:               config,
		telemetrySettings:    set.TelemetrySettings,
		projectToEndpoint:    make(map[string]*otlpEndpoints),
		projectCreationQueue: make(chan projectCreationRequest, projectCreationQueueSize),
		workerCtx:            workerCtx,
		workerCancel:         workerCancel,
		rateLimiter:          newRateLimiter(),
	}

	state.attributeKey = config.Routing.ProjectFromAttribute
	if state.attributeKey == "" {
		state.attributeKey = DefaultAttributeForProject
	}

	state.projectMapping = config.Routing.AttributeToProjectMapping
	set.Logger.Info("Configured sentryexporter",
		zap.String("org", config.OrgSlug),
		zap.String("routing_attribute", state.attributeKey),
		zap.Bool("auto_create_projects", config.AutoCreateProjects))

	return state, nil
}

func newSentryExporter(state *endpointState, logger *zap.Logger) *sentryExporter {
	return &sentryExporter{
		logger: logger,
		state:  state,
	}
}

// pushTraceData takes an incoming OpenTelemetry trace, and forwards it to the Sentry OTLP endpoint.
func (e *sentryExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	return e.state.pushTraceData(ctx, e.logger, td)
}

func (s *endpointState) pushTraceData(ctx context.Context, logger *zap.Logger, td ptrace.Traces) error {
	if s.closing.Load() {
		return errExporterShuttingDown
	}
	if td.SpanCount() == 0 {
		return nil
	}

	return s.routeTracesByProject(ctx, logger, td)
}

// routeTracesByProject splits traces by project and sends each batch to the appropriate endpoint
func (s *endpointState) routeTracesByProject(ctx context.Context, logger *zap.Logger, td ptrace.Traces) error {
	projectGroups := make(map[projectRoutingKey]any)
	var droppedSpans int

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		rs := td.ResourceSpans().At(i)
		attrs := rs.Resource().Attributes()
		projectSlug := s.extractProjectSlug(attrs)

		if projectSlug == "" {
			droppedSpans += countSpansInResource(rs)
			continue
		}

		platform := s.extractPlatform(attrs)
		key := projectRoutingKey{slug: projectSlug, platform: platform}

		group, exists := projectGroups[key]
		if !exists {
			group = ptrace.NewTraces()
		}
		traces := group.(ptrace.Traces)
		rs.CopyTo(traces.ResourceSpans().AppendEmpty())
		projectGroups[key] = traces
	}

	if droppedSpans > 0 {
		logger.Warn("Dropping traces missing routing attribute",
			zap.String("attribute", s.attributeKey),
			zap.Int("dropped_spans", droppedSpans))
	}

	return s.routeByProject(
		ctx,
		logger,
		projectGroups,
		func(ctx context.Context, logger *zap.Logger, data any, endpoint *otlpEndpoints) error {
			return s.sendTracesToEndpoint(ctx, logger, data.(ptrace.Traces), endpoint)
		},
	)
}

// sendTracesToEndpoint sends traces to a specific endpoint
func (s *endpointState) sendTracesToEndpoint(ctx context.Context, logger *zap.Logger, td ptrace.Traces, endpoint *otlpEndpoints) error {
	request := ptraceotlp.NewExportRequestFromTraces(td)
	data, err := request.MarshalProto()
	if err != nil {
		return fmt.Errorf("failed to marshal traces: %w", err)
	}

	if err := s.sendOTLPData(ctx, logger, endpoint.PublicKey, dataCategoryTrace, endpoint.TracesURL, data, endpoint.AuthHeader); err != nil {
		return fmt.Errorf("failed to send traces to Sentry: %w", err)
	}

	logger.Debug("Successfully sent traces to Sentry",
		zap.Int("span_count", td.SpanCount()))

	return nil
}

// projectCreationWorker runs in the background to handle async project creation
func (s *endpointState) projectCreationWorker() {
	defer s.workerWg.Done()
	s.telemetrySettings.Logger.Info("Starting project creation worker")

	for {
		select {
		case <-s.workerCtx.Done():
			s.telemetrySettings.Logger.Info("Project creation worker shutting down")
			return
		case req := <-s.projectCreationQueue:
			s.telemetrySettings.Logger.Debug("Processing project creation request",
				zap.String("project", req.projectSlug),
				zap.String("platform", req.platform))

			keepPending := false
			if req.projectSlug == "" || s.defaultTeamSlug == "" {
				s.telemetrySettings.Logger.Warn("Skipping invalid project creation request",
					zap.String("project", req.projectSlug),
					zap.String("team", s.defaultTeamSlug))
			} else {
				s.projectMapMu.RLock()
				_, exists := s.projectToEndpoint[req.projectSlug]
				s.projectMapMu.RUnlock()
				if exists {
					s.telemetrySettings.Logger.Debug("Project already exists, skipping creation",
						zap.String("project", req.projectSlug))
				} else {
					createCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
					_, err := s.sentryClient.CreateProject(createCtx, s.config.OrgSlug, s.defaultTeamSlug, req.projectSlug, req.projectSlug, req.platform)
					cancel()

					if err != nil {
						var apiErr *sentryHTTPError
						if errors.As(err, &apiErr) && apiErr.statusCode == http.StatusTooManyRequests {
							retryAfter := internalrl.DefaultRetryAfter
							if apiErr.headers != nil {
								if reset := apiErr.headers.Get("X-Sentry-Rate-Limit-Reset"); reset != "" {
									retryAfter = parseXSentryRateLimitReset(reset)
								}
							}
							s.telemetrySettings.Logger.Warn("Rate limited while creating project, will retry",
								zap.String("project", req.projectSlug),
								zap.Duration("retry_after", retryAfter))

							if retryAfter > 0 {
								time.Sleep(retryAfter)
							}
							select {
							case s.projectCreationQueue <- req:
								keepPending = true
							default:
								s.telemetrySettings.Logger.Warn("Failed to re-queue rate-limited project creation",
									zap.String("project", req.projectSlug))
							}
						} else {
							s.telemetrySettings.Logger.Error("Failed to create project asynchronously",
								zap.String("project", req.projectSlug),
								zap.Error(err))
						}
					} else {
						endpointCtx, endpointCancel := context.WithTimeout(context.Background(), 30*time.Second)
						endpoint, err := s.sentryClient.GetOTLPEndpoints(endpointCtx, s.config.OrgSlug, req.projectSlug)
						endpointCancel()

						if err != nil {
							var apiErr *sentryHTTPError
							if errors.As(err, &apiErr) && apiErr.statusCode == http.StatusTooManyRequests {
								retryAfter := internalrl.DefaultRetryAfter
								if apiErr.headers != nil {
									if reset := apiErr.headers.Get("X-Sentry-Rate-Limit-Reset"); reset != "" {
										retryAfter = parseXSentryRateLimitReset(reset)
									}
								}
								s.telemetrySettings.Logger.Warn("Rate limited while fetching endpoints, will retry",
									zap.String("project", req.projectSlug),
									zap.Duration("retry_after", retryAfter))

								if retryAfter > 0 {
									time.Sleep(retryAfter)
								}
								select {
								case s.projectCreationQueue <- req:
									keepPending = true
								default:
									s.telemetrySettings.Logger.Warn("Failed to re-queue rate-limited endpoint fetch",
										zap.String("project", req.projectSlug))
								}
							} else {
								s.telemetrySettings.Logger.Error("Failed to get endpoints for newly created project",
									zap.String("project", req.projectSlug),
									zap.Error(err))
							}
						} else {
							s.projectMapMu.Lock()
							s.projectToEndpoint[req.projectSlug] = endpoint
							s.projectMapMu.Unlock()

							s.telemetrySettings.Logger.Info("Successfully created project asynchronously",
								zap.String("project", req.projectSlug),
								zap.String("team", s.defaultTeamSlug),
								zap.String("platform", req.platform))
						}
					}
				}
			}

			if !keepPending {
				s.pendingCreations.Delete(req.projectSlug)
			}
		}
	}
}

// Start starts the exporter
func (s *endpointState) Start(ctx context.Context, host component.Host) error {
	s.startOnce.Do(func() {
		s.telemetrySettings.Logger.Info("Starting sentryexporter",
			zap.String("org", s.config.OrgSlug))

		client, err := newHTTPClient(ctx, s.config.ClientConfig, host, s.telemetrySettings)
		if err != nil {
			s.startErr = fmt.Errorf("failed to create HTTP client: %w", err)
			return
		}
		s.client = client
		s.sentryClient = newSentryClient(
			s.config.URL,
			string(s.config.AuthToken),
			s.client,
		)

		s.workerWg.Add(1)
		go s.projectCreationWorker()

		projects, err := s.sentryClient.GetAllProjects(ctx, s.config.OrgSlug)
		if err != nil {
			s.telemetrySettings.Logger.Warn("Failed to pre-populate project cache",
				zap.Error(err))
			return
		}

		if s.defaultTeamSlug == "" {
			for _, project := range projects {
				if len(project.Teams) > 0 {
					s.defaultTeamSlug = project.Teams[0].Slug
					break
				}
			}
		}

		s.preloadEndpointsFromOrgKeys(ctx, projects)
	})

	return s.startErr
}

func (s *endpointState) preloadEndpointsFromOrgKeys(ctx context.Context, projects []projectInfo) {
	keys, err := s.sentryClient.GetOrgProjectKeys(ctx, s.config.OrgSlug)
	if err != nil {
		s.telemetrySettings.Logger.Warn("Failed to preload project endpoints from org project keys",
			zap.Error(err))
		return
	}

	projectKeyByID := make(map[int]projectKey)
	for _, key := range keys {
		if !key.IsActive {
			continue
		}
		if _, exists := projectKeyByID[key.ProjectID]; !exists {
			projectKeyByID[key.ProjectID] = key
		}
	}

	if len(projectKeyByID) == 0 {
		return
	}

	s.projectMapMu.Lock()
	defer s.projectMapMu.Unlock()

	for _, project := range projects {
		if s.defaultTeamSlug == "" && len(project.Teams) > 0 {
			s.defaultTeamSlug = project.Teams[0].Slug
		}

		if _, ok := s.projectToEndpoint[project.Slug]; ok {
			continue
		}

		projectID := parseProjectID(project.ID)
		if projectID <= 0 {
			s.telemetrySettings.Logger.Warn("Skipping project due to invalid project ID",
				zap.String("project", project.Slug))
			continue
		}

		key, ok := projectKeyByID[projectID]
		if !ok {
			continue
		}

		dsn, err := key.DSN.ParsePublicDSN()
		if err != nil {
			s.telemetrySettings.Logger.Warn("Failed to parse DSN for project",
				zap.String("project", project.Slug),
				zap.Error(err))
			continue
		}

		s.projectToEndpoint[project.Slug] = &otlpEndpoints{
			TracesURL:  fmt.Sprintf("%s://%s/api/%s/integration/otlp/v1/traces/", dsn.GetScheme(), dsn.GetHost(), dsn.GetProjectID()),
			LogsURL:    fmt.Sprintf("%s://%s/api/%s/integration/otlp/v1/logs/", dsn.GetScheme(), dsn.GetHost(), dsn.GetProjectID()),
			PublicKey:  key.Public,
			AuthHeader: fmt.Sprintf("sentry sentry_key=%s", key.Public),
		}
	}
}

// Shutdown stops the exporter
func (s *endpointState) Shutdown(_ context.Context) error {
	var shutdownErr error
	s.shutdownOnce.Do(func() {
		s.closing.Store(true)
		if s.workerCancel != nil {
			s.workerCancel()
		}

		close(s.projectCreationQueue)

		done := make(chan struct{})
		go func() {
			s.workerWg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(timeout):
			s.telemetrySettings.Logger.Warn("Timed out waiting for project creation worker to stop")
		}

		if s.client != nil {
			s.client.CloseIdleConnections()
		}

		s.telemetrySettings.Logger.Info("Sentryexporter shutdown complete")
	})
	return shutdownErr
}

func (e *sentryExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	return e.state.pushLogData(ctx, e.logger, ld)
}

func (s *endpointState) pushLogData(ctx context.Context, logger *zap.Logger, ld plog.Logs) error {
	if s.closing.Load() {
		return errExporterShuttingDown
	}
	if ld.LogRecordCount() == 0 {
		return nil
	}

	return s.routeLogsByProject(ctx, logger, ld)
}

func (s *endpointState) routeLogsByProject(ctx context.Context, logger *zap.Logger, ld plog.Logs) error {
	projectGroups := make(map[projectRoutingKey]any)
	var droppedLogs int

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		rl := ld.ResourceLogs().At(i)
		attrs := rl.Resource().Attributes()
		projectSlug := s.extractProjectSlug(attrs)

		if projectSlug == "" {
			droppedLogs += countLogsInResource(rl)
			continue
		}

		platform := s.extractPlatform(attrs)
		key := projectRoutingKey{slug: projectSlug, platform: platform}

		group, exists := projectGroups[key]
		if !exists {
			group = plog.NewLogs()
		}
		logs := group.(plog.Logs)
		rl.CopyTo(logs.ResourceLogs().AppendEmpty())
		projectGroups[key] = logs
	}

	if droppedLogs > 0 {
		logger.Warn("Dropping logs missing routing attribute",
			zap.String("attribute", s.attributeKey),
			zap.Int("dropped_log_records", droppedLogs))
	}

	return s.routeByProject(
		ctx,
		logger,
		projectGroups,
		func(ctx context.Context, logger *zap.Logger, data any, endpoint *otlpEndpoints) error {
			return s.sendLogsToEndpoint(ctx, logger, data.(plog.Logs), endpoint)
		},
	)
}

func (s *endpointState) routeByProject(ctx context.Context, logger *zap.Logger, projectGroups map[projectRoutingKey]any, send func(context.Context, *zap.Logger, any, *otlpEndpoints) error) error {
	var errs error
	var projectsBeingCreated int

	for key, data := range projectGroups {
		if err := ctx.Err(); err != nil {
			return err
		}

		endpoint, err := s.getOrCreateProjectEndpoint(ctx, logger, key.slug, key.platform)
		if err != nil {
			if errors.Is(err, errProjectBeingCreated) {
				projectsBeingCreated++
				continue
			}
			logger.Error("Failed to get endpoint for project",
				zap.String("project", key.slug),
				zap.Error(err))
			errs = errors.Join(errs, err)
			continue
		}

		if err := send(ctx, logger, data, endpoint); err != nil {
			var httpErr *sentryHTTPError
			if errors.As(err, &httpErr) {
				if httpErr.statusCode == http.StatusForbidden && strings.Contains(httpErr.body, "event submission rejected with_reason: ProjectId") {
					logger.Warn("Project may have been deleted, removing from cache and retrying",
						zap.String("project", key.slug),
						zap.Int("status_code", httpErr.statusCode))
					s.projectMapMu.Lock()
					delete(s.projectToEndpoint, key.slug)
					s.projectMapMu.Unlock()

					newEndpoint, retryErr := s.getOrCreateProjectEndpoint(ctx, logger, key.slug, key.platform)
					if retryErr != nil {
						if errors.Is(retryErr, errProjectBeingCreated) {
							projectsBeingCreated++
							continue
						}
						logger.Error("Failed to get endpoint for project on retry",
							zap.String("project", key.slug),
							zap.Error(retryErr))
						errs = errors.Join(errs, retryErr)
						continue
					}

					if retryErr := send(ctx, logger, data, newEndpoint); retryErr != nil {
						logger.Error("Failed to send data to project on retry",
							zap.String("project", key.slug),
							zap.Error(retryErr))
						errs = errors.Join(errs, retryErr)
						continue
					}
					continue
				}
			}

			logger.Error("Failed to send data to project",
				zap.String("project", key.slug),
				zap.Error(err))
			errs = errors.Join(errs, err)
		}
	}

	if projectsBeingCreated > 0 {
		logger.Warn("Data dropped for projects being created asynchronously, future data will succeed",
			zap.Int("projects_being_created", projectsBeingCreated))
	}

	if errs != nil {
		return consumererror.NewPermanent(errs)
	}
	return nil
}

func countSpansInResource(rs ptrace.ResourceSpans) int {
	total := 0
	scopeSpans := rs.ScopeSpans()
	for i := 0; i < scopeSpans.Len(); i++ {
		total += scopeSpans.At(i).Spans().Len()
	}
	return total
}

func countLogsInResource(rl plog.ResourceLogs) int {
	total := 0
	scopeLogs := rl.ScopeLogs()
	for i := 0; i < scopeLogs.Len(); i++ {
		total += scopeLogs.At(i).LogRecords().Len()
	}
	return total
}

func (s *endpointState) sendLogsToEndpoint(ctx context.Context, logger *zap.Logger, ld plog.Logs, endpoint *otlpEndpoints) error {
	request := plogotlp.NewExportRequestFromLogs(ld)
	data, err := request.MarshalProto()
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	if err := s.sendOTLPData(ctx, logger, endpoint.PublicKey, dataCategoryLog, endpoint.LogsURL, data, endpoint.AuthHeader); err != nil {
		return fmt.Errorf("failed to send logs to Sentry: %w", err)
	}

	logger.Debug("Successfully sent logs to Sentry",
		zap.Int("log_count", ld.LogRecordCount()))

	return nil
}

// sentryHTTPError represents an HTTP error from Sentry
type sentryHTTPError struct {
	statusCode int
	body       string
	headers    http.Header
}

func (e *sentryHTTPError) Error() string {
	return fmt.Sprintf("request failed with status %d: %s", e.statusCode, e.body)
}

func (s *endpointState) sendOTLPData(ctx context.Context, logger *zap.Logger, dsn string, category dataCategory, endpoint string, data []byte, authHeader string) error {
	if delay, limited := s.rateLimiter.isRateLimited(dsn, category, time.Now()); limited {
		logger.Warn("Dropping data due to active rate limit",
			zap.String("endpoint", endpoint),
			zap.String("category", string(category)),
			zap.Duration("retry_after", delay))
		return exporterhelper.NewThrottleRetry(errRateLimited, delay)
	}

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(data); err != nil {
		return fmt.Errorf("failed to gzip request: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("failed to finalize gzip request: %w", err)
	}

	compressedData := compressed.Bytes()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(compressedData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("x-sentry-auth", authHeader)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	now := time.Now()
	s.rateLimiter.updateFromResponse(dsn, resp, now)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &sentryHTTPError{
			statusCode: resp.StatusCode,
			body:       string(body),
			headers:    resp.Header,
		}
	}

	logger.Debug("Successfully sent data to Sentry",
		zap.Int("status_code", resp.StatusCode),
		zap.Int("data_size", len(data)),
		zap.Int("compressed_size", len(compressedData)))

	return nil
}

func (s *endpointState) extractProjectSlug(attrs pcommon.Map) string {
	attrValue, exists := attrs.Get(s.attributeKey)
	if !exists {
		return ""
	}

	if attrValue.Type() != pcommon.ValueTypeStr {
		return ""
	}

	serviceName := attrValue.Str()
	if serviceName == "" {
		return ""
	}

	if s.projectMapping != nil {
		if mappedSlug, ok := s.projectMapping[serviceName]; ok {
			return mappedSlug
		}
	}

	return serviceName
}

func (*endpointState) extractPlatform(_ pcommon.Map) string {
	// for correctly mapping `telemetry.sdk.language` to the Sentry platform we need to keep track of all supported platforms.
	// defaulting to `other` for now to avoid validation issues.
	return "other"
}

func (s *endpointState) getOrCreateProjectEndpoint(ctx context.Context, logger *zap.Logger, projectSlug, platform string) (*otlpEndpoints, error) {
	s.projectMapMu.RLock()
	if cached, ok := s.projectToEndpoint[projectSlug]; ok {
		s.projectMapMu.RUnlock()
		return cached, nil
	}
	s.projectMapMu.RUnlock()

	result, err, _ := s.inflight.Do(projectSlug, func() (any, error) {
		s.projectMapMu.RLock()
		if cached, ok := s.projectToEndpoint[projectSlug]; ok {
			s.projectMapMu.RUnlock()
			return cached, nil
		}
		s.projectMapMu.RUnlock()

		endpoint, fetchErr := s.sentryClient.GetOTLPEndpoints(ctx, s.config.OrgSlug, projectSlug)
		if fetchErr == nil {
			s.projectMapMu.Lock()
			s.projectToEndpoint[projectSlug] = endpoint
			s.projectMapMu.Unlock()
			return endpoint, nil
		}

		if !s.config.AutoCreateProjects {
			return nil, fmt.Errorf("project %s not found and auto_create_projects is disabled", projectSlug)
		}

		if s.defaultTeamSlug == "" {
			return nil, fmt.Errorf("no team available for creating project %s", projectSlug)
		}

		s.projectMapMu.RLock()
		projectCount := len(s.projectToEndpoint)
		s.projectMapMu.RUnlock()

		if projectCount >= maxProjects {
			return nil, fmt.Errorf("maximum number of projects (%d) reached, cannot create project %s", maxProjects, projectSlug)
		}

		if _, pending := s.pendingCreations.LoadOrStore(projectSlug, struct{}{}); pending {
			return nil, errProjectBeingCreated
		}

		select {
		case s.projectCreationQueue <- projectCreationRequest{
			projectSlug: projectSlug,
			platform:    platform,
		}:
			logger.Info("Queued project for async creation",
				zap.String("project", projectSlug),
				zap.String("platform", platform))
		default:
			s.pendingCreations.Delete(projectSlug)
			return nil, fmt.Errorf("project creation queue is full for project %s", projectSlug)
		}

		return nil, errProjectBeingCreated
	})

	if err != nil {
		return nil, err
	}
	return result.(*otlpEndpoints), nil
}
