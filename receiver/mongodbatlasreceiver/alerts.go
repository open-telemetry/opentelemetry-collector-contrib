// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
)

// maxContentLength is the maximum payload size we will accept from incoming requests.
// Requests are generally ~1000 bytes, so we overshoot that by an order of magnitude.
// This is to protect from overly large requests.
const (
	maxContentLength    int64  = 16384
	signatureHeaderName string = "X-MMS-Signature"

	alertModeListen    alertMode = "listen"
	alertModeRetrieval alertMode = "retrieve"
	alertCacheKey                = "logging_alerts_cache"

	defaultAlertsPollInterval = 30 * time.Second
	defaultAlertsTTL          = 30 * time.Minute
)

type alertMode string

type alertsClient interface {
	GetProject(context.Context, string) (*mongodbatlas.Project, error)
	GetAlerts(context.Context, string) ([]mongodbatlas.Alert, error)
}

type alertsReceiver struct {
	addr        string
	secret      string
	server      *http.Server
	mode        alertMode
	tlsSettings *configtls.TLSServerSetting
	consumer    consumer.Logs
	wg          *sync.WaitGroup
	logger      *zap.Logger

	// only relevant in `retrieval` mode
	projects      []ProjectConfig
	client        alertsClient
	privateKey    string
	publicKey     string
	retrySettings exporterhelper.RetrySettings
	pollInterval  time.Duration
	cache         *alertCache
	doneChan      chan bool
	id            config.ComponentID  // ID of the receiver component
	storageID     *config.ComponentID // ID of the storage extension component
	storageClient *storage.Client
}

func newAlertsReceiver(logger *zap.Logger, baseConfig *Config, consumer consumer.Logs) (*alertsReceiver, error) {
	cfg := baseConfig.Alerts
	var tlsConfig *tls.Config

	if cfg.TLS != nil {
		var err error

		tlsConfig, err = cfg.TLS.LoadTLSConfig()
		if err != nil {
			return nil, err
		}
	}

	recv := &alertsReceiver{
		addr:          cfg.Endpoint,
		secret:        cfg.Secret,
		tlsSettings:   cfg.TLS,
		consumer:      consumer,
		mode:          alertMode(cfg.Mode),
		projects:      cfg.Projects,
		retrySettings: baseConfig.RetrySettings,
		publicKey:     baseConfig.PublicKey,
		privateKey:    baseConfig.PrivateKey,
		wg:            &sync.WaitGroup{},
		pollInterval:  baseConfig.Alerts.PollInterval,
		doneChan:      make(chan bool, 1),
		logger:        logger,
		id:            baseConfig.ID(),
		storageID:     baseConfig.StorageID,
	}

	if recv.mode == alertModeRetrieval {
		client, err := internal.NewMongoDBAtlasClient(recv.publicKey, recv.privateKey, recv.retrySettings, recv.logger)
		if err != nil {
			return nil, err
		}
		recv.client = client
	} else {
		s := &http.Server{
			TLSConfig: tlsConfig,
			Handler:   http.HandlerFunc(recv.handleRequest),
		}
		recv.server = s
	}

	return recv, nil
}

func (a alertsReceiver) Start(ctx context.Context, host component.Host) error {
	switch a.mode {
	case alertModeListen:
		return a.startListening(ctx, host)
	case alertModeRetrieval:
		return a.startRetrieving(ctx, host)
	default:
		return fmt.Errorf("receiver attempted to start without a known mode: %s", a.mode)
	}
}

func (a alertsReceiver) startRetrieving(ctx context.Context, host component.Host) error {
	a.logger.Debug("starting alerts receiver in retrieval mode")
	storageClient, err := adapter.GetStorageClient(ctx, host, a.storageID, a.id)
	if err != nil {
		return fmt.Errorf("failed to set up storage: %w", err)
	}
	a.storageClient = &storageClient
	err = a.syncPersistence(ctx)
	if err != nil {
		return fmt.Errorf("there was an error syncing the receiver with checkpoint: %w", err)
	}

	t := time.NewTicker(a.pollInterval)
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			select {
			case <-t.C:
				if err := a.retrieveAndProcessAlerts(ctx); err != nil {
					a.logger.Error("unable to retrieve alerts", zap.Error(err))
				}
			case <-a.doneChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (a alertsReceiver) retrieveAndProcessAlerts(ctx context.Context) error {
	var alerts []mongodbatlas.Alert
	for _, p := range a.projects {
		project, err := a.client.GetProject(ctx, p.Name)
		if err != nil {
			a.logger.Error("error retrieving project "+p.Name+":", zap.Error(err))
			continue
		}
		projectAlerts, err := a.client.GetAlerts(ctx, project.ID)
		if err != nil {
			a.logger.Error("unable to get alerts for project", zap.Error(err))
			continue
		}
		alerts = append(alerts, projectAlerts...)
	}
	now := pcommon.NewTimestampFromTime(time.Now())
	logs, err := a.convertAlerts(now, alerts)
	if err != nil {
		return err
	}
	if logs.LogRecordCount() > 0 {
		return a.consumer.ConsumeLogs(ctx, logs)
	}
	return a.writeCheckpoint(ctx)
}

func (a alertsReceiver) startListening(ctx context.Context, host component.Host) error {
	a.logger.Debug("starting alerts receiver in listening mode")
	// We use a.server.Serve* over a.server.ListenAndServe*
	// So that we can catch and return errors relating to binding to network interface on start.
	var lc net.ListenConfig

	l, err := lc.Listen(ctx, "tcp", a.addr)
	if err != nil {
		return err
	}

	a.wg.Add(1)
	if a.tlsSettings != nil {
		go func() {
			defer a.wg.Done()

			a.logger.Debug("Starting ServeTLS",
				zap.String("address", a.addr),
				zap.String("certfile", a.tlsSettings.CertFile),
				zap.String("keyfile", a.tlsSettings.KeyFile))

			err := a.server.ServeTLS(l, a.tlsSettings.CertFile, a.tlsSettings.KeyFile)

			a.logger.Debug("Serve TLS done")

			if err != http.ErrServerClosed {
				a.logger.Error("ServeTLS failed", zap.Error(err))
				host.ReportFatalError(err)
			}
		}()
	} else {
		go func() {
			defer a.wg.Done()

			a.logger.Debug("Starting Serve", zap.String("address", a.addr))

			err := a.server.Serve(l)

			a.logger.Debug("Serve done")

			if err != http.ErrServerClosed {
				a.logger.Error("Serve failed", zap.Error(err))
				host.ReportFatalError(err)
			}
		}()
	}
	return nil
}

func (a alertsReceiver) handleRequest(rw http.ResponseWriter, req *http.Request) {
	if req.ContentLength < 0 {
		rw.WriteHeader(http.StatusLengthRequired)
		a.logger.Debug("Got request with no Content-Length specified", zap.String("remote", req.RemoteAddr))
		return
	}

	if req.ContentLength > maxContentLength {
		rw.WriteHeader(http.StatusRequestEntityTooLarge)
		a.logger.Debug("Got request with large Content-Length specified",
			zap.String("remote", req.RemoteAddr),
			zap.Int64("content-length", req.ContentLength),
			zap.Int64("max-content-length", maxContentLength))
		return
	}

	payloadSigHeader := req.Header.Get(signatureHeaderName)
	if payloadSigHeader == "" {
		rw.WriteHeader(http.StatusBadRequest)
		a.logger.Debug("Got payload with no HMAC signature, dropping...")
		return
	}

	payload := make([]byte, req.ContentLength)
	_, err := io.ReadFull(req.Body, payload)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		a.logger.Debug("Failed to read alerts payload", zap.Error(err), zap.String("remote", req.RemoteAddr))
		return
	}

	if err = verifyHMACSignature(a.secret, payload, payloadSigHeader); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		a.logger.Debug("Got payload with invalid HMAC signature, dropping...", zap.Error(err), zap.String("remote", req.RemoteAddr))
		return
	}

	logs, err := payloadToLogs(time.Now(), payload)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		a.logger.Error("Failed to convert log payload to log record", zap.Error(err))
		return
	}

	if err := a.consumer.ConsumeLogs(req.Context(), logs); err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		a.logger.Error("Failed to consumer alert as log", zap.Error(err))
		return
	}

	rw.WriteHeader(http.StatusOK)
}

func (a alertsReceiver) Shutdown(ctx context.Context) error {
	switch a.mode {
	case alertModeListen:
		return a.shutdownListener(ctx)
	case alertModeRetrieval:
		return a.shutdownRetriever(ctx)
	default:
		return errors.New("alert receiver mode not set, unable to shutdown")
	}
}

func (a alertsReceiver) shutdownListener(ctx context.Context) error {
	a.logger.Debug("Shutting down server")
	err := a.server.Shutdown(ctx)
	if err != nil {
		return err
	}

	a.logger.Debug("Waiting for shutdown to complete.")
	a.wg.Wait()
	return nil
}

func (a alertsReceiver) shutdownRetriever(ctx context.Context) error {
	a.logger.Debug("Shutting down client")
	close(a.doneChan)
	a.wg.Wait()
	return a.writeCheckpoint(ctx)
}

func (a alertsReceiver) convertAlerts(now pcommon.Timestamp, alerts []mongodbatlas.Alert) (plog.Logs, error) {
	logs := plog.NewLogs()

	var errs error
	for _, alert := range alerts {
		if a.hasProcessed(alert) {
			continue
		}

		resourceLogs := logs.ResourceLogs().AppendEmpty()
		logRecord := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		logRecord.SetObservedTimestamp(now)

		ts, err := time.Parse(time.RFC3339, alert.Updated)
		if err != nil {
			a.logger.Warn("unable to interpret updated time for alert, expecting a RFC3339 timestamp", zap.String("timestamp", alert.Updated))
			continue
		}

		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		logRecord.SetSeverityNumber(severityFromAPIAlert(alert))
		// this could be fairly expensive to do, expecting not too many issues unless there are a ton
		// of unrecognized alerts to process.
		bodyBytes, err := json.Marshal(alert)
		if err != nil {
			a.logger.Warn("unable to marshal alert into a body string")
			continue
		}

		logRecord.Body().SetStringVal(string(bodyBytes))

		resourceAttrs := resourceLogs.Resource().Attributes()
		resourceAttrs.PutString("mongodbatlas.group.id", alert.GroupID)
		resourceAttrs.PutString("mongodbatlas.alert.config.id", alert.AlertConfigID)
		putStringToMapNotNil(resourceAttrs, "mongodbatlas.cluster.name", &alert.ClusterName)
		putStringToMapNotNil(resourceAttrs, "mongodbatlas.replica_set.name", &alert.ReplicaSetName)

		attrs := logRecord.Attributes()
		// These attributes are always present
		attrs.PutString("event.domain", "mongodbatlas")
		attrs.PutString("event.name", alert.EventTypeName)
		attrs.PutString("status", alert.Status)
		attrs.PutString("created", alert.Created)
		attrs.PutString("updated", alert.Updated)
		attrs.PutString("id", alert.ID)

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
			attrs.PutString("metric.units", alert.CurrentValue.Units)
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

		attrs.PutString("net.peer.name", host)
		attrs.PutInt("net.peer.port", port)
		a.cache.Put(alertKey(alert), cachePutOptions{
			ExpiresAt: now.AsTime().Add(30 * time.Minute),
		})
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
	logRecord.Body().SetStringVal(string(payload))

	resourceAttrs := resourceLogs.Resource().Attributes()
	resourceAttrs.PutString("mongodbatlas.group.id", alert.GroupID)
	resourceAttrs.PutString("mongodbatlas.alert.config.id", alert.AlertConfigID)
	putStringToMapNotNil(resourceAttrs, "mongodbatlas.cluster.name", alert.ClusterName)
	putStringToMapNotNil(resourceAttrs, "mongodbatlas.replica_set.name", alert.ReplicaSetName)

	attrs := logRecord.Attributes()
	// These attributes are always present
	attrs.PutString("event.domain", "mongodbatlas")
	attrs.PutString("event.name", alert.EventType)
	attrs.PutString("message", alert.HumanReadable)
	attrs.PutString("status", alert.Status)
	attrs.PutString("created", alert.Created)
	attrs.PutString("updated", alert.Updated)
	attrs.PutString("id", alert.ID)

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
		attrs.PutString("metric.units", alert.CurrentValue.Units)
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

		attrs.PutString("net.peer.name", host)
		attrs.PutInt("net.peer.port", port)

	}

	return logs, nil
}

type cachePutOptions struct {
	ExpiresAt time.Time `mapstructure:"expires_at"`
}

// alertCache wraps a sync Map so it is goroutine safe as well as
// can have custom marshaling
type alertCache struct {
	sync.Mutex
	Data map[string]cachePutOptions `mapstructure:"data"`
}

func (a *alertCache) Has(k string) bool {
	a.Lock()
	val, ok := a.Data[k]
	a.Unlock()
	if ok {
		now := time.Now()
		if now.After(val.ExpiresAt) {
			delete(a.Data, k)
			return false
		}
	}
	return ok
}

func (a *alertCache) Put(k string, opts cachePutOptions) {
	a.Lock()
	a.Data[k] = opts
	a.Unlock()
}

func (a *alertsReceiver) syncPersistence(ctx context.Context) error {
	if a.storageClient == nil {
		return nil
	}
	c := *a.storageClient
	cBytes, err := c.Get(ctx, alertCacheKey)
	if err != nil {
		return err
	}

	if cBytes != nil {
		var cache alertCache
		if err = json.Unmarshal(cBytes, &cache); err != nil {
			return fmt.Errorf("unable to decode stored cache: %w", err)
		}
		a.cache = &cache
		return nil
	}

	// create if not already loaded
	newC := &alertCache{
		Data: map[string]cachePutOptions{},
	}
	a.cache = newC

	return nil
}

func (a *alertsReceiver) writeCheckpoint(ctx context.Context) error {
	if a.storageClient == nil {
		return nil
	}
	c := *a.storageClient
	marshalBytes, err := json.Marshal(&a.cache)
	if err != nil {
		return fmt.Errorf("unable to write checkpoint: %w", err)
	}
	return c.Set(ctx, alertCacheKey, marshalBytes)
}

func (a *alertsReceiver) hasProcessed(alert mongodbatlas.Alert) bool {
	hasItem := a.cache.Has(alertKey(alert))
	return hasItem
}

// alertKey is a key to index the alert to see if it has already been processed.
func alertKey(alert mongodbatlas.Alert) string {
	return fmt.Sprintf("%s|%s|%s", alert.ID, alert.Status, alert.Updated)
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
func severityFromAPIAlert(a mongodbatlas.Alert) plog.SeverityNumber {
	switch a.Status {
	case "OPEN":
		return plog.SeverityNumberWarn
	default:
		return plog.SeverityNumberInfo
	}
}

func putStringToMapNotNil(m pcommon.Map, k string, v *string) {
	if v != nil {
		m.PutString(k, *v)
	}
}
