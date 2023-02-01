package mongodbatlasreceiver

import (
	"context"
	"encoding/json"
	"errors"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
)

const (
	eventStorageKey       = "last_recorded_event"
	defaultEventsMaxPages = 50
	defaultPollInterval   = time.Minute
)

type eventsClient interface {
	GetProject(ctx context.Context, groupID string) (*mongodbatlas.Project, error)
	GetEvents(ctx context.Context, groupID string, opts *internal.GetEventsOptions) (ret []*mongodbatlas.Event, nextPage bool, err error)
}

type eventsReceiver struct {
	client        eventsClient
	logger        *zap.Logger
	id            component.ID  // ID of the receiver component
	storageID     *component.ID // ID of the storage extension component
	storageClient storage.Client
	cfg           *Config
	consumer      consumer.Logs

	maxPages     int
	pollInterval time.Duration
	wg           *sync.WaitGroup
	record       *eventRecord
	doneChan     chan bool
}

type eventRecord struct {
	sync.Mutex
	LastRecordedEvent *time.Time `mapstructure:"last_recorded_event"`
}

func newEventsReceiver(settings rcvr.CreateSettings, c *Config, consumer consumer.Logs) *eventsReceiver {
	for _, p := range c.Events.Projects {
		p.populateIncludesAndExcludes()
	}

	r := &eventsReceiver{
		client:       internal.NewMongoDBAtlasClient(c.PublicKey, c.PrivateKey, c.RetrySettings, settings.Logger),
		cfg:          c,
		logger:       settings.Logger,
		id:           settings.ID,
		storageID:    c.StorageID,
		consumer:     consumer,
		pollInterval: c.Events.PollInterval,
		wg:           &sync.WaitGroup{},
		doneChan:     make(chan bool, 1),
	}

	if r.maxPages == 0 {
		r.maxPages = defaultEventsMaxPages
	}

	if r.pollInterval == 0 {
		r.pollInterval = time.Minute
	}

	return r
}

func (er *eventsReceiver) Start(ctx context.Context, host component.Host) error {
	er.logger.Debug("Starting up events receiver")
	return er.startPolling(ctx, host)
}

func (er *eventsReceiver) Shutdown(ctx context.Context) error {
	er.logger.Debug("Shutting down events receiver")
	close(er.doneChan)
	er.wg.Wait()
	return er.checkpoint(ctx)
}

func (er *eventsReceiver) startPolling(ctx context.Context, host component.Host) error {
	storageClient, err := adapter.GetStorageClient(ctx, host, er.storageID, er.id)
	if err != nil {
		return fmt.Errorf("failed to set up storage: %w", err)
	}
	er.storageClient = storageClient
	err = er.loadCheckpoint(ctx)

	t := time.NewTicker(er.pollInterval)
	er.wg.Add(1)
	go func() {
		defer er.wg.Done()
		for {
			select {
			case <-t.C:
				if err = er.pollEvents(ctx); err != nil {
					er.logger.Error("error while polling for events", zap.Error(err))
				}
			case <-er.doneChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (er *eventsReceiver) pollEvents(ctx context.Context) error {
	for _, p := range er.cfg.Events.Projects {
		project, err := er.client.GetProject(ctx, p.Name)
		if err != nil {
			er.logger.Error("error retrieving project information for "+p.Name+":", zap.Error(err))
			return err
		}
		er.poll(ctx, project, p)
	}

	return er.checkpoint(ctx)
}

func (er *eventsReceiver) poll(ctx context.Context, project *mongodbatlas.Project, p *ProjectConfig) {
	pollTime := time.Now()
	for pageN := 1; pageN <= er.maxPages; pageN++ {
		opts := &internal.GetEventsOptions{
			PageNum:    pageN,
			EventTypes: er.cfg.Events.Types,
			MaxDate:    pollTime,
		}

		if er.record.LastRecordedEvent != nil {
			opts.MinDate = *er.record.LastRecordedEvent
		} else {
			opts.MinDate = time.Now().Add(-er.pollInterval)
		}

		projectEvents, hasNext, err := er.client.GetEvents(ctx, project.ID, opts)
		if err != nil {
			er.logger.Error("unable to get events for project", zap.Error(err), zap.String("project", p.Name))
			break
		}

		now := pcommon.NewTimestampFromTime(pollTime)
		logs, err := er.transformEvents(now, projectEvents, project)
		if err != nil {
			er.logger.Error("error parsing events", zap.Error(err))
		}

		if logs.LogRecordCount() > 0 {
			if err = er.consumer.ConsumeLogs(ctx, logs); err != nil {
				er.logger.Error("error consuming events", zap.Error(err))
				break
			}
		}

		if !hasNext {
			break
		}
	}

}

func (er *eventsReceiver) transformEvents(now pcommon.Timestamp, events []*mongodbatlas.Event, p *mongodbatlas.Project) (plog.Logs, error) {
	logs := plog.NewLogs()
	var errs error
	for _, event := range events {
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		ra := resourceLogs.Resource().Attributes()
		ra.PutStr("mongodbatlas.project.name", p.Name)
		ra.PutStr("mongodbatlas.org.id", p.OrgID)
		ra.PutStr("mongodbatlas.group.id", event.GroupID)

		if event.ReplicaSetName != "" {
			ra.PutStr("mongodbatlas.replica_set.name", event.ReplicaSetName)
		}

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
			continue
		}
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		logRecord.SetObservedTimestamp(now)

		attrs := logRecord.Attributes()
		// always present attributes
		attrs.PutStr("event.domain", "mongodbatlas")
		attrs.PutStr("type", event.EventTypeName)
		attrs.PutStr("id", event.ID)

		parseOptionalAttributes(&attrs, event)
	}

	return logs, errs
}

func (er *eventsReceiver) checkpoint(ctx context.Context) error {
	if er.storageClient == nil {
		return errors.New("missing non-nil storage client")
	}
	marshalBytes, err := json.Marshal(er.record)
	if err != nil {
		return fmt.Errorf("unable to write checkpoint: %w", err)
	}
	return er.storageClient.Set(ctx, eventStorageKey, marshalBytes)
}

func (er *eventsReceiver) loadCheckpoint(ctx context.Context) error {
	if er.storageClient == nil {
		return nil
	}
	cBytes, err := er.storageClient.Get(ctx, eventStorageKey)
	if err != nil || cBytes == nil {
		er.record = &eventRecord{}
		return nil
	}

	var record eventRecord
	if err = json.Unmarshal(cBytes, &record); err != nil {
		return fmt.Errorf("unable to decode stored record for events: %w", err)
	}
	er.record = &record
	return nil
}

func parseOptionalAttributes(m *pcommon.Map, event *mongodbatlas.Event) {
	if event.AlertID != "" {
		m.PutStr("alert.id", event.AlertID)
	}

	if event.AlertConfigID != "" {
		m.PutStr("mongodbatlas.alert.config.id", event.AlertConfigID)
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
		m.PutStr("remote_address", event.RemoteAddress)
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

	if event.CurrentValue != nil {
		m.PutDouble("metric.value", *event.CurrentValue.Number)
		m.PutStr("metric.units", event.CurrentValue.Units)
	}

	if event.ShardName != "" {
		m.PutStr("shard.name", event.ShardName)
	}

}
