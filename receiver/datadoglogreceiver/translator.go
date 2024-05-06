// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadoglogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadoglogreceiver"

import (
	"encoding/json"
	"errors"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"net/http"
	"strings"
	"time"
)

var (
	ErrNoLogsInPayload = errors.New("no logs in datadog payload")
)

type commonResourceAttributes struct {
	origin      string
	ApiKey      string
	mwSource    string
	host        string
	serviceName string
}

type datadogLogMessage struct {
	Log    string `json:"log"`
	Stream string `json:"stream"`
	Time   string `json:"time"`
}

func getOtlpExportReqFromDatadogV2Logs(key string,
	logReq *HTTPLogItem) (plogotlp.ExportRequest, error) {

	if logReq == nil {
		return plogotlp.ExportRequest{}, ErrNoLogsInPayload
	}

	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs()
	rm := resourceLogs.AppendEmpty()
	resourceAttributes := rm.Resource().Attributes()

	logHost := logReq.Hostname
	commonResourceAttributes := commonResourceAttributes{
		origin:      logReq.Ddsource,
		ApiKey:      key,
		mwSource:    "datadog",
		host:        logHost,
		serviceName: logReq.Service,
	}

	setResourceAttributes(resourceAttributes, commonResourceAttributes)

	scopeLogs := rm.ScopeLogs().AppendEmpty()
	instrumentationScope := scopeLogs.Scope()
	instrumentationScope.SetName("datadog")
	instrumentationScope.SetVersion("v0.0.1")

	scopeLog := scopeLogs.LogRecords().AppendEmpty()
	logAttributes := scopeLog.Attributes()

	tagString := logReq.Ddtags
	tagList := strings.Split(tagString, ",")
	for _, tag := range tagList {
		keyVal := strings.Split(tag, ":")
		if len(keyVal) != 2 {
			continue
		}
		logAttributes.PutStr(keyVal[0], keyVal[1])
	}

	logMessage := logReq.Message

	scopeLog.Body().SetStr(logMessage)
	scopeLog.SetSeverityText(logReq.Status)
	logTime := time.Now()
	scopeLog.SetTimestamp(pcommon.Timestamp(logTime.UnixNano()))
	scopeLog.SetObservedTimestamp(pcommon.Timestamp(logTime.UnixNano()))
	return plogotlp.NewExportRequestFromLogs(logs), nil
}

func setResourceAttributes(attributes pcommon.Map,
	cra commonResourceAttributes) {
	if cra.serviceName != "" {
		attributes.PutStr("service.name", cra.serviceName)
	}
	if cra.ApiKey != "" {
		attributes.PutStr("mw.account_key", cra.ApiKey)
	}
	if cra.host != "" {
		attributes.PutStr("host.name", cra.host)
		attributes.PutStr("host.id", cra.host)
	}
	if cra.mwSource != "" {
		attributes.PutStr("source", cra.mwSource)
	}
}

func toLogs(log HTTPLogItem, req *http.Request) (plogotlp.ExportRequest, error) {
	var otlpReq plogotlp.ExportRequest
	var err error
	key := req.Header.Get("dd-api-key")
	otlpReq, err = getOtlpExportReqFromDatadogV2Logs(key, &log)
	if err != nil {
		return plogotlp.ExportRequest{}, err
	}
	return otlpReq, nil
}

func handlePayload(body []byte) (tp []HTTPLogItem, err error) {
	var v2Logs []HTTPLogItem
	err = json.Unmarshal(body, &v2Logs)
	if err != nil {
		return nil, err
	}
	return v2Logs, nil
}
