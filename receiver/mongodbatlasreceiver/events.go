// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
)

const (
	eventStorageKey       = "last_recorded_event"
	defaultEventsMaxPages = 25
	defaultEventsPageSize = 100
	defaultPollInterval   = time.Minute
)

type eventsClient interface {
	GetProject(ctx context.Context, groupID string) (*mongodbatlas.Project, error)
	GetProjectEvents(ctx context.Context, groupID string, opts *internal.GetEventsOptions) (ret []*mongodbatlas.Event, nextPage bool, err error)
	GetOrganization(ctx context.Context, orgID string) (*mongodbatlas.Organization, error)
	GetOrganizationEvents(ctx context.Context, orgID string, opts *internal.GetEventsOptions) (ret []*mongodbatlas.Event, nextPage bool, err error)
}

type eventsReceiver struct {
	client        eventsClient
	logger        *zap.Logger
	storageClient storage.Client
	cfg           *Config
	consumer      consumer.Logs

	maxPages     int
	pageSize     int
	pollInterval time.Duration
	wg           *sync.WaitGroup
	record       *eventRecord // this record is used for checkpointing last processed events
	cancel       context.CancelFunc
}

type eventRecord struct {
	NextStartTime *time.Time `mapstructure:"next_start_time"`
}

func newEventsReceiver(settings rcvr.CreateSettings, c *Config, consumer consumer.Logs) *eventsReceiver {
	r := &eventsReceiver{
		client:        internal.NewMongoDBAtlasClient(c.PublicKey, c.PrivateKey, c.RetrySettings, settings.Logger),
		cfg:           c,
		logger:        settings.Logger,
		consumer:      consumer,
		pollInterval:  c.Events.PollInterval,
		wg:            &sync.WaitGroup{},
		maxPages:      int(c.Events.MaxPages),
		pageSize:      int(c.Events.PageSize),
		storageClient: storage.NewNopClient(),
	}

	if r.maxPages == 0 {
		r.maxPages = defaultEventsMaxPages
	}

	if r.pageSize == 0 {
		r.pageSize = defaultEventsPageSize
	}

	if r.pollInterval == 0 {
		r.pollInterval = time.Minute
	}

	return r
}

func (er *eventsReceiver) Start(ctx context.Context, host component.Host, storageClient storage.Client) error {
	er.logger.Debug("Starting up events receiver")
	cancelCtx, cancel := context.WithCancel(ctx)
	er.cancel = cancel
	er.storageClient = storageClient
	er.loadCheckpoint(cancelCtx)

	return er.startPolling(cancelCtx)
}

func (er *eventsReceiver) Shutdown(ctx context.Context) error {
	er.logger.Debug("Shutting down events receiver")
	er.cancel()
	er.wg.Wait()
	return er.checkpoint(ctx)
}

func (er *eventsReceiver) startPolling(ctx context.Context) error {
	t := time.NewTicker(er.pollInterval)
	er.wg.Add(1)
	go func() {
		defer er.wg.Done()
		for {
			select {
			case <-t.C:
				if err := er.pollEvents(ctx); err != nil {
					er.logger.Error("error while polling for events", zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (er *eventsReceiver) pollEvents(ctx context.Context) error {
	st := pcommon.NewTimestampFromTime(time.Now().Add(-er.pollInterval)).AsTime()
	if er.record.NextStartTime != nil {
		st = *er.record.NextStartTime
	}
	et := time.Now()

	for _, pc := range er.cfg.Events.Projects {
		project, err := er.client.GetProject(ctx, pc.Name)
		if err != nil {
			er.logger.Error("error retrieving project information for "+pc.Name+":", zap.Error(err))
			return err
		}
		er.pollProject(ctx, project, pc, st, et)
	}

	for _, pc := range er.cfg.Events.Organizations {
		org, err := er.client.GetOrganization(ctx, pc.ID)
		if err != nil {
			er.logger.Error("error retrieving org information for "+pc.ID+":", zap.Error(err))
			return err
		}
		er.pollOrg(ctx, org, pc, st, et)
	}

	er.record.NextStartTime = &et
	return er.checkpoint(ctx)
}

func (er *eventsReceiver) pollProject(ctx context.Context, project *mongodbatlas.Project, p *ProjectConfig, startTime, now time.Time) {
	for pageN := 1; pageN <= er.maxPages; pageN++ {
		opts := &internal.GetEventsOptions{
			PageNum:    pageN,
			EventTypes: er.cfg.Events.Types,
			MaxDate:    now,
			MinDate:    startTime,
		}

		projectEvents, hasNext, err := er.client.GetProjectEvents(ctx, project.ID, opts)
		if err != nil {
			er.logger.Error("unable to get events for project", zap.Error(err), zap.String("project", p.Name))
			break
		}

		now := pcommon.NewTimestampFromTime(now)
		logs := er.transformProjectEvents(now, projectEvents, project)

		if logs.LogRecordCount() > 0 {
			if err = er.consumer.ConsumeLogs(ctx, logs); err != nil {
				er.logger.Error("error consuming project events", zap.Error(err))
				break
			}
		}

		if !hasNext {
			break
		}
	}
}

func (er *eventsReceiver) pollOrg(ctx context.Context, org *mongodbatlas.Organization, p *OrgConfig, startTime, now time.Time) {
	for pageN := 1; pageN <= er.maxPages; pageN++ {
		opts := &internal.GetEventsOptions{
			PageNum:    pageN,
			EventTypes: er.cfg.Events.Types,
			MaxDate:    now,
			MinDate:    startTime,
		}

		organizationEvents, hasNext, err := er.client.GetOrganizationEvents(ctx, org.ID, opts)
		if err != nil {
			er.logger.Error("unable to get events for organization", zap.Error(err), zap.String("organization", p.ID))
			break
		}

		now := pcommon.NewTimestampFromTime(now)
		logs := er.transformOrgEvents(now, organizationEvents, org)

		if logs.LogRecordCount() > 0 {
			if err = er.consumer.ConsumeLogs(ctx, logs); err != nil {
				er.logger.Error("error consuming organization events", zap.Error(err))
				break
			}
		}

		if !hasNext {
			break
		}
	}
}

func (er *eventsReceiver) transformProjectEvents(now pcommon.Timestamp, events []*mongodbatlas.Event, p *mongodbatlas.Project) plog.Logs {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	ra := resourceLogs.Resource().Attributes()
	ra.PutStr("mongodbatlas.project.name", p.Name)
	ra.PutStr("mongodbatlas.org.id", p.OrgID)
	er.transformEvents(now, events, &resourceLogs)
	return logs
}

func (er *eventsReceiver) transformOrgEvents(now pcommon.Timestamp, events []*mongodbatlas.Event, o *mongodbatlas.Organization) plog.Logs {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	ra := resourceLogs.Resource().Attributes()
	ra.PutStr("mongodbatlas.org.id", o.ID)
	er.transformEvents(now, events, &resourceLogs)
	return logs
}

func (er *eventsReceiver) transformEvents(now pcommon.Timestamp, events []*mongodbatlas.Event, resourceLogs *plog.ResourceLogs) {
	for _, event := range events {

		logRecord := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		bodyBytes, err := json.Marshal(event)
		if err != nil {
			er.logger.Error("unable to unmarshal event into body string", zap.Error(err))
			continue
		}
		logRecord.Body().SetStr(string(bodyBytes))

		// ISO-8601 formatted
		ts, err := time.Parse(time.RFC3339, event.Created)
		if err != nil {
			er.logger.Warn("unable to interpret when an event was created, expecting a RFC3339 timestamp", zap.String("timestamp", event.Created), zap.String("event", event.ID))
			logRecord.SetTimestamp(now)
		} else {
			logRecord.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		}
		logRecord.SetObservedTimestamp(now)

		attrs := logRecord.Attributes()
		// always present attributes
		attrs.PutStr("event.domain", "mongodbatlas")
		attrs.PutStr("type", event.EventTypeName)
		attrs.PutStr("id", event.ID)
		attrs.PutStr("group.id", event.GroupID)

		parseOptionalAttributes(&attrs, event)
	}
}

func (er *eventsReceiver) checkpoint(ctx context.Context) error {
	marshalBytes, err := json.Marshal(er.record)
	if err != nil {
		return fmt.Errorf("unable to write checkpoint: %w", err)
	}
	return er.storageClient.Set(ctx, eventStorageKey, marshalBytes)
}

func (er *eventsReceiver) loadCheckpoint(ctx context.Context) {
	cBytes, err := er.storageClient.Get(ctx, eventStorageKey)
	if err != nil {
		er.logger.Info("unable to load checkpoint from storage client, continuing without a previous checkpoint", zap.Error(err))
		er.record = &eventRecord{}
		return
	}

	if cBytes == nil {
		er.record = &eventRecord{}
		return
	}

	var record eventRecord
	if err = json.Unmarshal(cBytes, &record); err != nil {
		er.logger.Error("unable to decode stored record for events, continuing without a checkpoint", zap.Error(err))
		er.record = &eventRecord{}
		return
	}
	er.record = &record
}

func parseOptionalAttributes(m *pcommon.Map, event *mongodbatlas.Event) {
	if event.AlertID != "" {
		m.PutStr("alert.id", event.AlertID)
	}

	if event.AlertConfigID != "" {
		m.PutStr("alert.config.id", event.AlertConfigID)
	}

	if event.Collection != "" {
		m.PutStr("collection", event.Collection)
	}

	if event.Database != "" {
		m.PutStr("database", event.Database)
	}

	if event.Hostname != "" {
		m.PutStr("net.peer.name", event.Hostname)
	}

	if event.Port != 0 {
		m.PutInt("net.peer.port", int64(event.Port))
	}

	if event.InvoiceID != "" {
		m.PutStr("invoice.id", event.InvoiceID)
	}

	if event.Username != "" {
		m.PutStr("user.name", event.Username)
	}

	if event.TargetUsername != "" {
		m.PutStr("target.user.name", event.TargetUsername)
	}

	if event.UserID != "" {
		m.PutStr("user.id", event.UserID)
	}

	if event.TeamID != "" {
		m.PutStr("team.id", event.TeamID)
	}

	if event.RemoteAddress != "" {
		m.PutStr("remote.ip", event.RemoteAddress)
	}

	if event.MetricName != "" {
		m.PutStr("metric.name", event.MetricName)
	}

	if event.OpType != "" {
		m.PutStr("event.op_type", event.OpType)
	}

	if event.PaymentID != "" {
		m.PutStr("payment.id", event.PaymentID)
	}

	if event.ReplicaSetName != "" {
		m.PutStr("replica_set.name", event.ReplicaSetName)
	}

	if event.CurrentValue != nil {
		m.PutDouble("metric.value", *event.CurrentValue.Number)
		m.PutStr("metric.units", event.CurrentValue.Units)
	}

	if event.ShardName != "" {
		m.PutStr("shard.name", event.ShardName)
	}
}
