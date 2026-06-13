// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package activedirectoryinvreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectoryinvreceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type adReceiver struct {
	config   *ADConfig
	logger   *zap.Logger
	client   Client
	runtime  RuntimeInfo
	consumer consumer.Logs
	wg       *sync.WaitGroup
	doneChan chan bool
}

// newLogsReceiver creates a new Active Directory Inventory receiver
func newLogsReceiver(cfg *ADConfig, logger *zap.Logger, client Client, runtime RuntimeInfo, consumer consumer.Logs) *adReceiver {
	return &adReceiver{
		config:   cfg,
		logger:   logger,
		client:   client,
		runtime:  runtime,
		consumer: consumer,
		wg:       &sync.WaitGroup{},
		doneChan: make(chan bool),
	}
}

// Start the logs receiver
func (l *adReceiver) Start(ctx context.Context, _ component.Host) error {
	if !l.runtime.SupportedOS() {
		return errSupportedOS
	}
	l.logger.Debug("Starting to poll for active directory inventory records")
	l.wg.Add(1)
	go l.startPolling(ctx)
	return nil
}

// Shutdown the logs receiver
func (l *adReceiver) Shutdown(_ context.Context) error {
	l.logger.Debug("Shutting down logs receiver")
	close(l.doneChan)
	l.wg.Wait()
	return nil
}

// Start polling for Active Directory inventory records
func (l *adReceiver) startPolling(ctx context.Context) {
	defer l.wg.Done()
	l.logger.Info("Polling interval: ", zap.Duration("interval", l.config.PollInterval))
	// initial poll before starting the ticker
	err := l.poll(ctx)
	if err != nil {
		l.logger.Error("there was an error during the poll", zap.Error(err))
	}
	t := time.NewTicker(l.config.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.doneChan:
			return
		case <-t.C:
			err := l.poll(ctx)
			if err != nil {
				l.logger.Error("there was an error during the poll", zap.Error(err))
			}
		}
	}
}

// Traverse the Active Directory tree and set user attributes to log records
func (l *adReceiver) traverse(node Container, attrs []string, resourceLogs *plog.ResourceLogs) {
	nodeObject, err := node.ToObject()
	if err != nil {
		l.logger.Error("Failed to convert container to object", zap.Error(err))
		return
	}
	err = setConfiguredAttributes(nodeObject, attrs, resourceLogs)
	if err != nil {
		l.logger.Error("Failed to set configured attributes", zap.Error(err))
		return
	}
	children, err := node.Children()
	if err != nil {
		l.logger.Error("Failed to retrieve children", zap.Error(err))
		return
	}
	defer children.Close()
	for child, err := children.Next(); err == nil; child, err = children.Next() {
		childContainer, err := child.ToContainer()
		if err != nil {
			l.logger.Error("Failed to convert child object to container", zap.Error(err))
			return
		}
		l.traverse(childContainer, attrs, resourceLogs)
		childContainer.Close()
	}
}

// Poll for Active Directory inventory records
func (l *adReceiver) poll(ctx context.Context) error {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	resourceLogs := &rl
	_ = resourceLogs.ScopeLogs().AppendEmpty()
	root, err := l.client.Open(l.config.BaseDN)
	if err != nil {
		return fmt.Errorf("failed to open Active Directory path %q: %w", l.config.BaseDN, err)
	}
	defer root.Close()
	l.traverse(root, l.config.Attributes, resourceLogs)
	err = l.consumer.ConsumeLogs(ctx, logs)
	if err != nil {
		l.logger.Error("Error consuming log", zap.Error(err))
	}
	return nil
}

// Set configured attributes to a log record body
func setConfiguredAttributes(obj Object, attrs []string, resourceLogs *plog.ResourceLogs) error {
	observedTime := pcommon.NewTimestampFromTime(time.Now())
	attributes := make(map[string]any)
	for _, attr := range attrs {
		values, err := obj.Attrs(attr)
		if err == nil && len(values) > 0 {
			if len(values) == 1 {
				attributes[attr] = values[0]
				continue
			}
			attributes[attr] = values
		}
	}
	attributesJSON, err := json.Marshal(attributes)
	if err != nil {
		return err
	}
	logRecord := resourceLogs.ScopeLogs().At(0).LogRecords().AppendEmpty()
	logRecord.SetObservedTimestamp(observedTime)
	logRecord.SetTimestamp(observedTime)
	logRecord.Body().SetStr(string(attributesJSON))
	return nil
}
