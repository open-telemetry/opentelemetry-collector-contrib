// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- SHA1 is the algorithm mongodbatlas uses, it must be used to calculate the HMAC signature
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
)

// maxContentLength is the maximum payload size we will accept from incoming requests.
// Requests are generally ~1000 bytes, so we overshoot that by an order of magnitude.
// This is to protect from overly large requests.
const (
	maxContentLength    int64  = 16384
	signatureHeaderName string = "X-MMS-Signature"

	alertModeListen = "listen"
	alertModePoll   = "poll"
	alertCacheKey   = "last_recorded_alert"

	defaultAlertsPollInterval = 5 * time.Minute
	// defaults were based off API docs https://www.mongodb.com/docs/atlas/reference/api/alerts-get-all-alerts/
	defaultAlertsPageSize = 100
	defaultAlertsMaxPages = 10
)

type alertsClient interface {
	GetProject(ctx context.Context, groupID string) (*mongodbatlas.Project, error)
	GetAlerts(ctx context.Context, groupID string, opts *internal.AlertPollOptions) ([]mongodbatlas.Alert, bool, error)
}

type alertsReceiver struct {
	addr        string
	secret      string
	server      *http.Server
	mode        string
	tlsSettings *configtls.ServerConfig
	consumer    consumer.Logs
	wg          *sync.WaitGroup

	// only relevant in `poll` mode
	projects          []*ProjectConfig
	client            alertsClient
	baseURL           string
	privateKey        string
	publicKey         string
	backoffConfig     configretry.BackOffConfig
	pollInterval      time.Duration
	record            *alertRecord
	pageSize          int64
	maxPages          int64
	doneChan          chan bool
	storageClient     storage.Client
	telemetrySettings component.TelemetrySettings
}

func newAlertsReceiver(params rcvr.Settings, baseConfig *Config, consumer consumer.Logs) (*alertsReceiver, error) {
	cfg := baseConfig.Alerts
	var tlsConfig *tls.Config

	if cfg.TLS != nil {
		var err error

		tlsConfig, err = cfg.TLS.LoadTLSConfig(context.Background())
		if err != nil {
			return nil, err
		}
	}

	for _, p := range cfg.Projects {
		p.populateIncludesAndExcludes()
	}

	recv := &alertsReceiver{
		addr:              cfg.Endpoint,
		secret:            string(cfg.Secret),
		tlsSettings:       cfg.TLS,
		consumer:          consumer,
		mode:              cfg.Mode,
		projects:          cfg.Projects,
		backoffConfig:     baseConfig.BackOffConfig,
		baseURL:           baseConfig.BaseURL,
		publicKey:         baseConfig.PublicKey,
		privateKey:        string(baseConfig.PrivateKey),
		wg:                &sync.WaitGroup{},
		pollInterval:      baseConfig.Alerts.PollInterval,
		maxPages:          baseConfig.Alerts.MaxPages,
		pageSize:          baseConfig.Alerts.PageSize,
		doneChan:          make(chan bool, 1),
		telemetrySettings: params.TelemetrySettings,
	}

	if recv.mode == alertModePoll {
		client, err := internal.NewMongoDBAtlasClient(recv.baseURL, recv.publicKey, recv.privateKey, recv.backoffConfig, recv.telemetrySettings.Logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create MongoDB Atlas client for alerts receiver: %w", err)
		}

		recv.client = client
		return recv, nil
	}
	s := &http.Server{
		TLSConfig:         tlsConfig,
		Handler:           http.HandlerFunc(recv.handleRequest),
		ReadHeaderTimeout: 20 * time.Second,
	}
	recv.server = s
	return recv, nil
}

func (a *alertsReceiver) Start(ctx context.Context, host component.Host, storageClient storage.Client) error {
	if a.mode == alertModePoll {
		return a.startPolling(ctx, storageClient)
	}
	return a.startListening(ctx, host)
}

func (a *alertsReceiver) startPolling(ctx context.Context, storageClient storage.Client) error {
	a.telemetrySettings.Logger.Debug("starting alerts receiver in retrieval mode")
	a.storageClient = storageClient
	err := a.syncPersistence(ctx)
	if err != nil {
		a.telemetrySettings.Logger.Error("there was an error syncing the receiver with checkpoint", zap.Error(err))
	}

	t := time.NewTicker(a.pollInterval)
	a.wg.Go(func() {
		for {
			select {
			case <-t.C:
				if err := a.retrieveAndProcessAlerts(ctx); err != nil {
					a.telemetrySettings.Logger.Error("unable to retrieve alerts", zap.Error(err))
				}
			case <-a.doneChan:
				return
			case <-ctx.Done():
				return
			}
		}
	})

	return nil
}

func (a *alertsReceiver) retrieveAndProcessAlerts(ctx context.Context) error {
	for _, p := range a.projects {
		project, err := a.client.GetProject(ctx, p.Name)
		if err != nil {
			a.telemetrySettings.Logger.Error("error retrieving project "+p.Name+":", zap.Error(err))
			continue
		}
		a.pollAndProcess(ctx, p, project)
	}
	return a.writeCheckpoint(ctx)
}

func (a *alertsReceiver) pollAndProcess(ctx context.Context, pc *ProjectConfig, project *mongodbatlas.Project) {
	for pageNum := 1; pageNum <= int(a.maxPages); pageNum++ {
		projectAlerts, hasNext, err := a.client.GetAlerts(ctx, project.ID, &internal.AlertPollOptions{
			PageNum:  pageNum,
			PageSize: int(a.pageSize),
		})
		if err != nil {
			a.telemetrySettings.Logger.Error("unable to get alerts for project", zap.Error(err))
			break
		}

		filteredAlerts := a.applyFilters(pc, projectAlerts)
		now := pcommon.NewTimestampFromTime(time.Now())
		logs, err := a.convertAlerts(now, filteredAlerts, project)
		if err != nil {
			a.telemetrySettings.Logger.Error("error processing alerts", zap.Error(err))
			break
		}

		if logs.LogRecordCount() > 0 {
			if err = a.consumer.ConsumeLogs(ctx, logs); err != nil {
				a.telemetrySettings.Logger.Error("error consuming alerts", zap.Error(err))
				break
			}
		}
		if !hasNext {
			break
		}
	}
}

func (a *alertsReceiver) startListening(ctx context.Context, host component.Host) error {
	a.telemetrySettings.Logger.Debug("starting alerts receiver in listening mode")
	// We use a.server.Serve* over a.server.ListenAndServe*
	// So that we can catch and return errors relating to binding to network interface on start.
	var lc net.ListenConfig

	l, err := lc.Listen(ctx, "tcp", a.addr)
	if err != nil {
		return err
	}

	if a.tlsSettings != nil {
		a.wg.Go(func() {
			a.telemetrySettings.Logger.Debug("Starting ServeTLS",
				zap.String("address", a.addr),
				zap.String("certfile", a.tlsSettings.CertFile),
				zap.String("keyfile", a.tlsSettings.KeyFile))

			err := a.server.ServeTLS(l, a.tlsSettings.CertFile, a.tlsSettings.KeyFile)

			a.telemetrySettings.Logger.Debug("Serve TLS done")

			if err != http.ErrServerClosed {
				a.telemetrySettings.Logger.Error("ServeTLS failed", zap.Error(err))
				componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
			}
		})
	} else {
		a.wg.Go(func() {
			a.telemetrySettings.Logger.Debug("Starting Serve", zap.String("address", a.addr))

			err := a.server.Serve(l)

			a.telemetrySettings.Logger.Debug("Serve done")

			if err != http.ErrServerClosed {
				componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
			}
		})
	}
	return nil
}

func (a *alertsReceiver) handleRequest(rw http.ResponseWriter, req *http.Request) {
	if req.ContentLength < 0 {
		rw.WriteHeader(http.StatusLengthRequired)
		a.telemetrySettings.Logger.Debug("Got request with no Content-Length specified", zap.String("remote", req.RemoteAddr))
		return
	}

	if req.ContentLength > maxContentLength {
		rw.WriteHeader(http.StatusRequestEntityTooLarge)
		a.telemetrySettings.Logger.Debug("Got request with large Content-Length specified",
			zap.String("remote", req.RemoteAddr),
			zap.Int64("content-length", req.ContentLength),
			zap.Int64("max-content-length", maxContentLength))
		return
	}

	payloadSigHeader := req.Header.Get(signatureHeaderName)
	if payloadSigHeader == "" {
		rw.WriteHeader(http.StatusBadRequest)
		a.telemetrySettings.Logger.Debug("Got payload with no HMAC signature, dropping...")
		return
	}

	payload := make([]byte, req.ContentLength)
	_, err := io.ReadFull(req.Body, payload)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		a.telemetrySettings.Logger.Debug("Failed to read alerts payload", zap.Error(err), zap.String("remote", req.RemoteAddr))
		return
	}

	if err = verifyHMACSignature(a.secret, payload, payloadSigHeader); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		a.telemetrySettings.Logger.Debug("Got payload with invalid HMAC signature, dropping...", zap.Error(err), zap.String("remote", req.RemoteAddr))
		return
	}

	logs, err := payloadToLogs(time.Now(), payload)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		a.telemetrySettings.Logger.Error("Failed to convert log payload to log record", zap.Error(err))
		return
	}

	if err := a.consumer.ConsumeLogs(req.Context(), logs); err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		a.telemetrySettings.Logger.Error("Failed to consumer alert as log", zap.Error(err))
		return
	}

	rw.WriteHeader(http.StatusOK)
}

func (a *alertsReceiver) Shutdown(ctx context.Context) error {
	if a.mode == alertModePoll {
		return a.shutdownPoller(ctx)
	}
	return a.shutdownListener(ctx)
}

func (a *alertsReceiver) shutdownListener(ctx context.Context) error {
	a.telemetrySettings.Logger.Debug("Shutting down server")
	err := a.server.Shutdown(ctx)
	if err != nil {
		return err
	}

	a.telemetrySettings.Logger.Debug("Waiting for shutdown to complete.")
	a.wg.Wait()
	return nil
}

func (a *alertsReceiver) shutdownPoller(ctx context.Context) error {
	a.telemetrySettings.Logger.Debug("Shutting down client")
	close(a.doneChan)
	a.wg.Wait()
	return a.writeCheckpoint(ctx)
}

func (a *alertsReceiver) convertAlerts(now pcommon.Timestamp, alerts []*mongodbatlas.Alert, project *mongodbatlas.Project) (plog.Logs, error) {
	logs := plog.NewLogs()
	var errs error
	for i := range alerts {
		alert := alerts[i]
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		resourceAttrs := resourceLogs.Resource().Attributes()
		resourceAttrs.PutStr("mongodbatlas.group.id", alert.GroupID)
		resourceAttrs.PutStr("mongodbatlas.alert.config.id", alert.AlertConfigID)
		resourceAttrs.PutStr("mongodbatlas.org.id", project.OrgID)
		resourceAttrs.PutStr("mongodbatlas.project.name", project.Name)
		putStringToMapNotNil(resourceAttrs, "mongodbatlas.cluster.name", &alert.ClusterName)
		putStringToMapNotNil(resourceAttrs, "mongodbatlas.replica_set.name", &alert.ReplicaSetName)

		logRecord := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		logRecord.SetObservedTimestamp(now)

		ts, err := time.Parse(time.RFC3339, alert.Updated)
		if err != nil {
			a.telemetrySettings.Logger.Warn("unable to interpret updated time for alert, expecting a RFC3339 timestamp", zap.String("timestamp", alert.Updated))
			continue
		}

		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		logRecord.SetSeverityNumber(severityFromAPIAlert(alert.Status))
		logRecord.SetSeverityText(alert.Status)
		// this could be fairly expensive to do, expecting not too many issues unless there are a ton
		// of unrecognized alerts to process.
		bodyBytes, err := json.Marshal(alert)
		if err != nil {
			a.telemetrySettings.Logger.Warn("unable to marshal alert into a body string")
			continue
		}

		logRecord.Body().SetStr(string(bodyBytes))

		attrs := logRecord.Attributes()
		// These attributes are always present
		attrs.PutStr("event.domain", "mongodbatlas")
		attrs.PutStr("event.name", alert.EventTypeName)
		attrs.PutStr("status", alert.Status)
		attrs.PutStr("created", alert.Created)
		attrs.PutStr("updated", alert.Updated)
		attrs.PutStr("id", alert.ID)

		// These attributes are optional and may not be present, depending on the alert type.
		putStringToMapNotNil(attrs, "metric.name", &alert.MetricName)
		putStringToMapNotNil(attrs, "type_name", &alert.EventTypeName)
		putStringToMapNotNil(attrs, "last_notified", &alert.LastNotified)
		putStringToMapNotNil(attrs, "resolved", &alert.Resolved)
		putStringToMapNotNil(attrs, "acknowledgement.comment", &alert.AcknowledgementComment)
		putStringToMapNotNil(attrs, "acknowledgement.username", &alert.AcknowledgingUsername)
		putStringToMapNotNil(attrs, "acknowledgement.until", &alert.AcknowledgedUntil)

		if alert.CurrentValue != nil {
			attrs.PutDouble("metric.value", *alert.CurrentValue.Number)
			attrs.PutStr("metric.units", alert.CurrentValue.Units)
		}

		// Only present for HOST, HOST_METRIC, and REPLICA_SET alerts
		if alert.HostnameAndPort == "" {
			continue
		}

		host, portStr, err := net.SplitHostPort(alert.HostnameAndPort)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to split host:port %s: %w", alert.HostnameAndPort, err))
			continue
		}

		port, err := strconv.ParseInt(portStr, 10, 64)
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to parse port %s: %w", portStr, err))
			continue
		}

		attrs.PutStr("net.peer.name", host)
		attrs.PutInt("net.peer.port", port)
	}
	return logs, errs
}

func verifyHMACSignature(secret string, payload []byte, signatureHeader string) error {
	b64Decoder := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(signatureHeader))
	payloadSig, err := io.ReadAll(b64Decoder)
	if err != nil {
		return err
	}

	h := hmac.New(sha1.New, []byte(secret))
	h.Write(payload)
	calculatedSig := h.Sum(nil)

	if !hmac.Equal(calculatedSig, payloadSig) {
		return errors.New("calculated signature does not equal header signature")
	}

	return nil
}

func payloadToLogs(now time.Time, payload []byte) (plog.Logs, error) {
	var alert model.Alert

	err := json.Unmarshal(payload, &alert)
	if err != nil {
		return plog.Logs{}, err
	}

	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	logRecord := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))
	logRecord.SetTimestamp(timestampFromAlert(alert))
	logRecord.SetSeverityNumber(severityFromAlert(alert))
	logRecord.Body().SetStr(string(payload))

	resourceAttrs := resourceLogs.Resource().Attributes()
	resourceAttrs.PutStr("mongodbatlas.group.id", alert.GroupID)
	resourceAttrs.PutStr("mongodbatlas.alert.config.id", alert.AlertConfigID)
	putStringToMapNotNil(resourceAttrs, "mongodbatlas.cluster.name", alert.ClusterName)
	putStringToMapNotNil(resourceAttrs, "mongodbatlas.replica_set.name", alert.ReplicaSetName)

	attrs := logRecord.Attributes()
	// These attributes are always present
	attrs.PutStr("event.domain", "mongodbatlas")
	attrs.PutStr("event.name", alert.EventType)
	attrs.PutStr("message", alert.HumanReadable)
	attrs.PutStr("status", alert.Status)
	attrs.PutStr("created", alert.Created)
	attrs.PutStr("updated", alert.Updated)
	attrs.PutStr("id", alert.ID)

	// These attributes are optional and may not be present, depending on the alert type.
	putStringToMapNotNil(attrs, "metric.name", alert.MetricName)
	putStringToMapNotNil(attrs, "type_name", alert.TypeName)
	putStringToMapNotNil(attrs, "user_alias", alert.UserAlias)
	putStringToMapNotNil(attrs, "last_notified", alert.LastNotified)
	putStringToMapNotNil(attrs, "resolved", alert.Resolved)
	putStringToMapNotNil(attrs, "acknowledgement.comment", alert.AcknowledgementComment)
	putStringToMapNotNil(attrs, "acknowledgement.username", alert.AcknowledgementUsername)
	putStringToMapNotNil(attrs, "acknowledgement.until", alert.AcknowledgedUntil)

	if alert.CurrentValue != nil {
		attrs.PutDouble("metric.value", alert.CurrentValue.Number)
		attrs.PutStr("metric.units", alert.CurrentValue.Units)
	}

	if alert.HostNameAndPort != nil {
		host, portStr, err := net.SplitHostPort(*alert.HostNameAndPort)
		if err != nil {
			return plog.Logs{}, fmt.Errorf("failed to split host:port %s: %w", *alert.HostNameAndPort, err)
		}

		port, err := strconv.ParseInt(portStr, 10, 64)
		if err != nil {
			return plog.Logs{}, fmt.Errorf("failed to parse port %s: %w", portStr, err)
		}

		attrs.PutStr("net.peer.name", host)
		attrs.PutInt("net.peer.port", port)
	}

	return logs, nil
}

// alertRecord wraps a sync Map so it is goroutine safe as well as
// can have custom marshaling
type alertRecord struct {
	sync.Mutex
	LastRecordedTime *time.Time `mapstructure:"last_recorded"`
}

func (a *alertRecord) SetLastRecorded(lastUpdated *time.Time) {
	a.Lock()
	a.LastRecordedTime = lastUpdated
	a.Unlock()
}

func (a *alertsReceiver) syncPersistence(ctx context.Context) error {
	if a.storageClient == nil {
		return nil
	}
	cBytes, err := a.storageClient.Get(ctx, alertCacheKey)
	if err != nil || cBytes == nil {
		a.record = &alertRecord{}
		return nil
	}

	var cache alertRecord
	if err = json.Unmarshal(cBytes, &cache); err != nil {
		return fmt.Errorf("unable to decode stored cache: %w", err)
	}
	a.record = &cache
	return nil
}

func (a *alertsReceiver) writeCheckpoint(ctx context.Context) error {
	if a.storageClient == nil {
		a.telemetrySettings.Logger.Error("unable to write checkpoint since no storage client was found")
		return errors.New("missing non-nil storage client")
	}
	marshalBytes, err := json.Marshal(&a.record)
	if err != nil {
		return fmt.Errorf("unable to write checkpoint: %w", err)
	}
	return a.storageClient.Set(ctx, alertCacheKey, marshalBytes)
}

func (a *alertsReceiver) applyFilters(pConf *ProjectConfig, alerts []mongodbatlas.Alert) []*mongodbatlas.Alert {
	filtered := []*mongodbatlas.Alert{}

	lastRecordedTime := pcommon.Timestamp(0).AsTime()
	if a.record.LastRecordedTime != nil {
		lastRecordedTime = *a.record.LastRecordedTime
	}
	// we need to maintain two timestamps in order to not conflict while iterating
	latestInPayload := pcommon.Timestamp(0).AsTime()

	for i := range alerts {
		alert := &alerts[i]
		updatedTime, err := time.Parse(time.RFC3339, alert.Updated)
		if err != nil {
			a.telemetrySettings.Logger.Warn("unable to interpret updated time for alert, expecting a RFC3339 timestamp", zap.String("timestamp", alert.Updated))
			continue
		}

		if updatedTime.Before(lastRecordedTime) || updatedTime.Equal(lastRecordedTime) {
			// already processed if the updated time was before or equal to the last recorded
			continue
		}

		if len(pConf.excludesByClusterName) > 0 {
			if _, ok := pConf.excludesByClusterName[alert.ClusterName]; ok {
				continue
			}
		}

		if len(pConf.IncludeClusters) > 0 {
			if _, ok := pConf.includesByClusterName[alert.ClusterName]; !ok {
				continue
			}
		}

		filtered = append(filtered, alert)
		if updatedTime.After(latestInPayload) {
			latestInPayload = updatedTime
		}
	}

	if latestInPayload.After(lastRecordedTime) {
		a.record.SetLastRecorded(&latestInPayload)
	}

	return filtered
}

func timestampFromAlert(a model.Alert) pcommon.Timestamp {
	if time, err := time.Parse(time.RFC3339, a.Updated); err == nil {
		return pcommon.NewTimestampFromTime(time)
	}

	return pcommon.Timestamp(0)
}

// severityFromAlert maps the alert to a severity number.
// Currently, it just maps "OPEN" alerts to WARN, and everything else to INFO.
func severityFromAlert(a model.Alert) plog.SeverityNumber {
	// Status is defined here: https://www.mongodb.com/docs/atlas/reference/api/alerts-get-alert/#response-elements
	// It may also be "INFORMATIONAL" for single-fire alerts (events)
	switch a.Status {
	case "OPEN":
		return plog.SeverityNumberWarn
	default:
		return plog.SeverityNumberInfo
	}
}

// severityFromAPIAlert is a workaround for shared types between the API and the model
func severityFromAPIAlert(a string) plog.SeverityNumber {
	switch a {
	case "OPEN":
		return plog.SeverityNumberWarn
	default:
		return plog.SeverityNumberInfo
	}
}

func putStringToMapNotNil(m pcommon.Map, k string, v *string) {
	if v != nil {
		m.PutStr(k, *v)
	}
}
