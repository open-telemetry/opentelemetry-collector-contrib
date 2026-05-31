// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package activedirectoryinvreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectoryinvreceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	adsi "github.com/go-adsi/adsi"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// Client is an interface for an Active Directory client
type Client interface {
	Open(path string, resourceLogs *plog.ResourceLogs) (Container, error)
}

type adsiClient struct{}

func (c *adsiClient) Open(path string, resourceLogs *plog.ResourceLogs) (Container, error) {
	client, err := adsi.NewClient()
	if err != nil {
		return nil, err
	}
	ldapPath := fmt.Sprintf("LDAP://%s", path)
	root, err := client.Open(ldapPath)
	if err != nil {
		return nil, err
	}
	rootContainer, err := root.ToContainer()
	if err != nil {
		return nil, err
	}
	windowsContainer := &adsiContainer{rootContainer}
	return windowsContainer, nil
}

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
	t := time.NewTicker(l.config.PollInterval)
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
func (r *adReceiver) traverse(node Container, attrs []string, resourceLogs *plog.ResourceLogs) {
	nodeObject, err := node.ToObject()
	if err != nil {
		r.logger.Error("Failed to convert container to object", zap.Error(err))
		return
	}
	err = setUserAttributes(nodeObject, attrs, resourceLogs)
	if err != nil {
		r.logger.Error("Failed to set user attributes", zap.Error(err))
		return
	}
	children, err := node.Children()
	if err != nil {
		r.logger.Error("Failed to retrieve children", zap.Error(err))
		return
	}
	for child, err := children.Next(); err == nil; child, err = children.Next() {
		windowsChildContainer, err := child.ToContainer()
		if err != nil {
			r.logger.Error("Failed to convert child object to container", zap.Error(err))
			return
		}
		childContainer := &adsiContainer{windowsChildContainer}
		r.traverse(childContainer, attrs, resourceLogs)
	}
	defer node.Close()
	children.Close()
}

// Poll for Active Directory inventory records
func (r *adReceiver) poll(ctx context.Context) error {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	resourceLogs := &rl
	_ = resourceLogs.ScopeLogs().AppendEmpty()
	root, err := r.client.Open(r.config.BaseDN, resourceLogs)
	if err != nil {
		return fmt.Errorf("invalid distinguished name, please verify that the domain exists: %w", err)
	}
	r.traverse(root, r.config.Attributes, resourceLogs)
	err = r.consumer.ConsumeLogs(ctx, logs)
	if err != nil {
		r.logger.Error("Error consuming log", zap.Error(err))
	}
	return nil
}

// Set user attributes to a log record body
func setUserAttributes(user Object, attrs []string, resourceLogs *plog.ResourceLogs) error {
	observedTime := pcommon.NewTimestampFromTime(time.Now())
	attributes := make(map[string]interface{})
	for _, attr := range attrs {
		values, err := user.Attrs(attr)
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
