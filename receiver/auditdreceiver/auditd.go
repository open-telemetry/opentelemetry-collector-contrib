// Copyright SAP Cloud Infrastructure
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package auditdreceiver // import "github.com/cloudoperators/opentelemetry-collector-contrib/receiver/auditdreceiver"

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-libaudit/v2"
	"github.com/elastic/go-libaudit/v2/auparse"
	"github.com/elastic/go-libaudit/v2/rule"
	"github.com/elastic/go-libaudit/v2/rule/flags"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

var (
	PATTERN                = regexp.MustCompile(`audit\((\d+)\.(\d+):(\d+)\):`)
	ErrorRuleAlreadyExists = "rule exists"
)

type Auditd struct {
	rules    []string
	client   *libaudit.AuditClient
	consumer consumer.Logs
	logger   *zap.Logger
	done     chan struct{}
}

func newAuditd(cfg *AuditdReceiverConfig, consumer consumer.Logs, settings receiver.Settings) (*Auditd, error) {
	return &Auditd{
		rules:    cfg.Rules,
		consumer: consumer,
		logger:   settings.Logger,
	}, nil
}

func createLogs(ts time.Time, messageType auparse.AuditMessageType, messageID int64, messageData []byte) plog.Logs {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	logSlice := resourceLogs.ScopeLogs().AppendEmpty().LogRecords()
	logRecord := logSlice.AppendEmpty()
	logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
	logRecord.SetSeverityText(plog.SeverityNumberInfo.String())
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	logRecord.Attributes().PutStr("type", messageType.String())
	logRecord.Attributes().PutInt("id", messageID)
	logRecord.Attributes().PutStr("data", string(messageData))
	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return logs
}

func (aud *Auditd) parseMessageDetails(data []byte) (int64, int64, int64) {
	if !PATTERN.Match(data) {
		aud.logger.Info("got a message without timestamp", zap.String("message", string(data)))
		return 0, 0, 0
	}
	groups := PATTERN.FindSubmatch(data)
	s, err := strconv.ParseInt(string(groups[1]), 10, 64)
	if err != nil {
		aud.logger.Error("could not parse timestamp", zap.String("s", string(groups[1])), zap.Error(err))
		s = 0
	}
	ns, err := strconv.ParseInt(string(groups[2]), 10, 64)
	if err != nil {
		aud.logger.Error("could not parse timestamp", zap.String("ns", string(groups[2])), zap.Error(err))
		ns = 0
	}
	id, err := strconv.ParseInt(string(groups[3]), 10, 64)
	if err != nil {
		aud.logger.Error("could not parse id", zap.String("id", string(groups[3])), zap.Error(err))
		id = 0
	}
	return s, ns, id
}

func (aud *Auditd) receive(ctx context.Context) {
	aud.logger.Info("starting listening for events")
	for {
		select {
		case _, ok := <-aud.done:
			_ = ok
			return
		case <-ctx.Done():
			return
		default:
			rawEvent, err := aud.client.Receive(false)
			if err != nil {
				aud.logger.Error("receive failed", zap.Error(err))
			}
			s, ns, id := aud.parseMessageDetails(rawEvent.Data)
			ts := time.Unix(s, ns)
			// Messages from 1100-2999 are valid audit messages.
			if rawEvent.Type < auparse.AUDIT_USER_AUTH ||
				rawEvent.Type > auparse.AUDIT_LAST_USER_MSG2 {
				continue
			}
			logs := createLogs(ts, rawEvent.Type, id, rawEvent.Data)
			aud.consumer.ConsumeLogs(ctx, logs)
		}
	}
}

func (aud *Auditd) prepareRules() error {
	for _, rawRule := range aud.rules {
		r, err := flags.Parse(rawRule)
		if err != nil {
			return fmt.Errorf("failed to parse rule %w", err)
		}
		wireRule, err := rule.Build(r)
		if err != nil {
			return fmt.Errorf("failed to build rule %w", err)
		}
		err = aud.client.AddRule(wireRule)
		if err != nil {
			if strings.Contains(err.Error(), ErrorRuleAlreadyExists) {
				aud.logger.Warn("rule already exists (skipping)", zap.Error(err))
			} else {
				return fmt.Errorf("failed to add rule: %v. (%v)", rawRule, err)
			}
		}
	}
	return nil
}

func (aud *Auditd) initAuditing() error {
	status, err := aud.client.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get audit status: %w", err)
	}
	if status.Enabled == 0 {
		aud.logger.Info("enabling auditing in the kernel")
		if err = aud.client.SetEnabled(true, libaudit.WaitForReply); err != nil {
			return fmt.Errorf("failed to set enabled=true: %w", err)
		}
	}
	aud.logger.Info("telling kernel this client should get the auditd logs")
	if err = aud.client.SetPID(libaudit.NoWait); err != nil {
		return fmt.Errorf("failed to set audit PID: %w", err)
	}

	return nil
}

func (aud *Auditd) Start(ctx context.Context, host component.Host) error {
	aud.done = make(chan struct{})
	var w io.Writer
	client, err := libaudit.NewAuditClient(w)
	if err != nil {
		return fmt.Errorf("failed to create client %w", err)
	}
	aud.client = client

	err = aud.prepareRules()
	if err != nil {
		return fmt.Errorf("failed to setup rules: %v", err)
	}

	err = aud.initAuditing()
	if err != nil {
		return fmt.Errorf("failed to initialise auditing: %v", err)
	}

	go aud.receive(ctx)
	return nil
}

func (aud *Auditd) Shutdown(_ context.Context) error {
	if aud.done != nil {
		aud.done <- struct{}{}
		close(aud.done)
		aud.client.Close()
		aud.done = nil
	}
	return nil
}
