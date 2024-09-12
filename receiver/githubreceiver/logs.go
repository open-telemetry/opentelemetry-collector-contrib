// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

var gitHubMaxLimit = 100

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
	CloseIdleConnections()
}

type gitHubLogsReceiver struct {
	client   httpClient
	logger   *zap.Logger
	consumer consumer.Logs
	cfg      *Config
	wg       *sync.WaitGroup
	cancel   context.CancelFunc
	nextURL  string
}

// newGitHubLogsReceiver creates a new GitHub logs receiver.
func newGitHubLogsReceiver(cfg *Config, logger *zap.Logger, consumer consumer.Logs) (*gitHubLogsReceiver, error) {
	return &gitHubLogsReceiver{
		cfg:      cfg,
		logger:   logger,
		consumer: consumer,
		wg:       &sync.WaitGroup{},
		client:   http.DefaultClient,
	}, nil
}

// start begins the receiver's operation.
func (r *gitHubLogsReceiver) Start(_ context.Context, _ component.Host) error {
	r.logger.Info("Starting GitHub logs receiver")
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Add(1)
	if r.cfg.PollInterval > 0 {
		// Start polling
		go r.startPolling(ctx)
	}
	return nil

}

func (r *gitHubLogsReceiver) setupWebhook() error {
	// waiting on open source code for webhooks: https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/34944/commits
	return nil
}

func (r *gitHubLogsReceiver) startPolling(ctx context.Context) {
	defer r.wg.Done()
	t := time.NewTicker(r.cfg.PollInterval)
	err := r.poll(ctx)
	if err != nil {
		r.logger.Error("there was an error during the first poll", zap.Error(err))
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			err := r.poll(ctx)
			if err != nil {
				r.logger.Error("there was an error during the poll", zap.Error(err))
			}
		}
	}
}

func (r *gitHubLogsReceiver) poll(ctx context.Context) error {
	logEvents := r.getLogs(ctx)
	observedTime := pcommon.NewTimestampFromTime(time.Now())
	logs := r.processLogEvents(observedTime, logEvents)
	if logs.LogRecordCount() > 0 {
		if err := r.consumer.ConsumeLogs(ctx, logs); err != nil {
			return err
		}
	}
	return nil
}

func (r *gitHubLogsReceiver) getLogs(ctx context.Context) []interface{} {
	pollTime := time.Now().UTC()
	token := r.cfg.AccessToken
	page := 1
	var url string
	var endpoint string
	var curLogs []interface{}
	var allLogs []interface{}

	// initialize log slice variable for unmarshalling
	var userLogs []gitHubUserLog
	var orgLogs []gitHubOrganizationLog
	var enterpriseLogs []gitHubEnterpriseLog

	// set endpoint based on log type
	switch r.cfg.LogType {
	case "user":
		endpoint = fmt.Sprintf("users/%s/events/public", r.cfg.Name)
	case "organization":
		endpoint = fmt.Sprintf("orgs/%s/audit-log", r.cfg.Name)
	default: // "enterprise"
		endpoint = fmt.Sprintf("enterprises/%s/audit-log", r.cfg.Name)
	}

	for {
		// use nextURL if it's set
		if r.nextURL != "" {
			url = r.nextURL
		} else {
			url = fmt.Sprintf("https://api.github.com/%s?per_page=%d&page=%d", endpoint, gitHubMaxLimit, page)
		}

		// create a new HTTP request with the provided context and URL
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			r.logger.Error("error creating request: %w", zap.Error(err))
			return allLogs
		}

		// add headers
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		req.Header.Add("Accept", "application/vnd.github.v3+json") // optional?
		req.Header.Add("X-GitHub-Api-Version", "2022-11-28")       // optional?

		resp, err := r.client.Do(req)
		if err != nil {
			r.logger.Error("error making request", zap.Error(err))
			return allLogs
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			r.logger.Error("unexpected status code", zap.Int("statusCode", resp.StatusCode))
			return allLogs
		}

		// read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			r.logger.Error("error reading response body", zap.Error(err))
			return allLogs
		}

		// unmarshal JSON into the appropriate log type
		var unmarshalErr error
		switch r.cfg.LogType {
		case "user":
			unmarshalErr = json.Unmarshal(body, &userLogs)
			curLogs = make([]interface{}, len(userLogs))
			for i, log := range userLogs {
				curLogs[i] = log
			}
		case "organization":
			unmarshalErr = json.Unmarshal(body, &orgLogs)
			curLogs = make([]interface{}, len(orgLogs))
			for i, log := range orgLogs {
				curLogs[i] = log
			}
		default: // "enterprise"
			unmarshalErr = json.Unmarshal(body, &enterpriseLogs)
			curLogs = make([]interface{}, len(enterpriseLogs))
			for i, log := range enterpriseLogs {
				curLogs[i] = log
			}
		}

		if unmarshalErr != nil {
			r.logger.Error("error unmarshalling JSON", zap.Error(unmarshalErr))
			return allLogs
		}

		// append current logs to allLogs
		allLogs = append(allLogs, curLogs...)

		page = r.setNextLink(resp, page)

		// check for the 'Link' header and set the next URL if it exists
		var curLogTime time.Time
		if len(curLogs) != 0 {
			switch r.cfg.LogType {
			case "user":
				curLogTime = curLogs[len(curLogs)-1].(gitHubUserLog).CreatedAt
			case "organization":
				curLogTime = millisecondsToTime(curLogs[len(curLogs)-1].(gitHubOrganizationLog).Timestamp)
			case "enterprise":
				curLogTime = millisecondsToTime(curLogs[len(curLogs)-1].(gitHubEnterpriseLog).Timestamp)
			}
		}

		if r.nextURL == "" || len(curLogs) < gitHubMaxLimit || curLogTime.After(pollTime) {
			break
		}
	}

	return allLogs
}

func (r *gitHubLogsReceiver) processLogEvents(observedTime pcommon.Timestamp, logEvents []interface{}) plog.Logs {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	resourceLogs.ScopeLogs().AppendEmpty()
	resourceAttributes := resourceLogs.Resource().Attributes()
	resourceAttributes.PutStr("log_type", r.cfg.LogType)
	resourceAttributes.PutStr("name", r.cfg.Name)

	for _, logEvent := range logEvents {
		logRecord := resourceLogs.ScopeLogs().At(0).LogRecords().AppendEmpty()
		logRecord.SetObservedTimestamp(observedTime)

		var timestamp time.Time
		switch event := logEvent.(type) {
		case *gitHubUserLog:
			timestamp = time.UnixMilli(event.CreatedAt.UnixNano() / int64(time.Millisecond))
		case *gitHubOrganizationLog:
			timestamp = time.UnixMilli(event.Timestamp)
		case *gitHubEnterpriseLog:
			timestamp = time.UnixMilli(event.Timestamp)
		}

		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))

		// body
		logEventBytes, err := json.Marshal(logEvent)
		if err != nil {
			r.logger.Error("unable to marshal logEvent", zap.Error(err))
		} else {
			logRecord.Body().SetStr(string(logEventBytes))
		}
		// add attributes

		switch r.cfg.LogType {
		case "user":
			r.addUserAttributes(logRecord, logEvent.(gitHubUserLog))
		case "organization":
			r.addOrganizationAttributes(logRecord, logEvent.(gitHubOrganizationLog))
		case "enterprise":
			r.addEnterpriseAttributes(logRecord, logEvent.(gitHubEnterpriseLog))
		}
	}
	return logs
}

// Helper function to add organization attributes to logRecord
func (r *gitHubLogsReceiver) addUserAttributes(logRecord plog.LogRecord, logEvent GitHubLog) {
	// add attributes
	attrs := logRecord.Attributes()
	attrs.PutStr("id", logEvent.(gitHubUserLog).ID)
	attrs.PutStr("type", logEvent.(gitHubUserLog).Type)
	attrs.PutInt("actor.id", logEvent.(gitHubUserLog).Actor.ID)
	attrs.PutStr("actor.login", logEvent.(gitHubUserLog).Actor.Login)
	attrs.PutStr("actor.display_login", logEvent.(gitHubUserLog).Actor.DisplayLogin)
	attrs.PutStr("actor.URL", logEvent.(gitHubUserLog).Actor.URL)
	attrs.PutInt("repo.id", logEvent.(gitHubUserLog).Repo.ID)

}

// Helper function to add organization attributes to logRecord
func (r *gitHubLogsReceiver) addOrganizationAttributes(logRecord plog.LogRecord, logEvent GitHubLog) {
	// add attributes
	attrs := logRecord.Attributes()
	attrs.PutStr("action", logEvent.(gitHubOrganizationLog).Action)
	attrs.PutStr("actor", logEvent.(gitHubOrganizationLog).Actor)
	attrs.PutInt("actor_id", logEvent.(gitHubOrganizationLog).ActorID)
	attrs.PutInt("created_at", logEvent.(gitHubOrganizationLog).CreatedAt)
}

// Helper function to add enterprise attributes to logRecord
func (r *gitHubLogsReceiver) addEnterpriseAttributes(logRecord plog.LogRecord, logEvent GitHubLog) {
	// add attributes
	attrs := logRecord.Attributes()
	attrs.PutStr("_document_id", logEvent.(gitHubEnterpriseLog).DocumentID)
	attrs.PutStr("action", logEvent.(gitHubEnterpriseLog).Action)
	attrs.PutStr("actor", logEvent.(gitHubEnterpriseLog).Actor)
	attrs.PutInt("actor_id", logEvent.(gitHubEnterpriseLog).ActorID)
	attrs.PutStr("actor_location", logEvent.(gitHubEnterpriseLog).ActorLocation.CountryCode)
	attrs.PutStr("business", logEvent.(gitHubEnterpriseLog).Business)
	attrs.PutInt("business_id", logEvent.(gitHubEnterpriseLog).BusinessID)
	attrs.PutInt("created_at", logEvent.(gitHubEnterpriseLog).CreatedAt)
	attrs.PutStr("operation_type", logEvent.(gitHubEnterpriseLog).OperationType)
	attrs.PutStr("user_agent", logEvent.(gitHubEnterpriseLog).UserAgent)
}

func (r *gitHubLogsReceiver) setNextLink(res *http.Response, page int) int {
	for _, link := range res.Header["Link"] {
		// split the link into URL and parameters
		parts := strings.Split(strings.TrimSpace(link), ";")
		if len(parts) < 2 {
			continue
		}

		// check if the "rel" parameter is "next"
		if strings.TrimSpace(parts[1]) == `rel="next"` {
			page++

			// extract and return the URL
			r.nextURL = strings.Trim(parts[0], "<>")

			return page
		}
	}
	r.logger.Error("unable to get next link")
	return page
}

func millisecondsToTime(ms int64) time.Time {
	return time.Unix(0, ms*int64(time.Millisecond))
}

func (r *gitHubLogsReceiver) Shutdown(_ context.Context) error {
	r.logger.Debug("shutting down logs receiver")
	if r.cancel != nil {
		r.cancel()
	}
	r.client.CloseIdleConnections()
	r.wg.Wait()
	return nil
}
