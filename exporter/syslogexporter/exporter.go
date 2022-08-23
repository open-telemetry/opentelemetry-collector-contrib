// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package syslogexporter contains an opentelemetry-collector exporter
// for syslog.
// nolint:errcheck
package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogsexporter"

//todo multicore
//todo flush and buffering
//todo retries for single log entry
//todo retry if bunch is errored
//todo reliability
//todo exporter metrics
//todo tls
//todo compress

import (
	"context"
	"github.com/influxdata/go-syslog/v3/rfc5424"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"strings"
	"time"
)

type syslogExporter struct {
	logger      *zap.Logger
	maxAttempts int
	client      sysLogClient
	sdIdConf    SdConfig
}

func (e *syslogExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var errs []error

	//todo merge all logs and scoped logs option
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resource := rl.Resource()
		ills := rl.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {

				syslog, err := toSyslogFormat(resource, logs.At(k), e.sdIdConf)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				if err := e.client.SendLog(syslog); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}

					errs = append(errs, err)
				}
			}
		}
	}
	return multierr.Combine(errs...)
}

func toSyslogFormat(resource pcommon.Resource, record plog.LogRecord, sdIdConf SdConfig) (string, error) {
	msg := &rfc5424.SyslogMessage{}
	msg.SetVersion(1) //always 1 https://datatracker.ietf.org/doc/html/rfc5424#section-9.1

	timestamp := record.Timestamp().AsTime().Format(time.RFC3339Nano)
	msg.SetTimestamp(timestamp)
	msg.SetPriority(uint8(record.SeverityNumber()))

	attributes := mergeAttributes(resource, record)
	addAttributes(attributes, sdIdConf, msg)
	addStandardFields(attributes, msg)

	addStaticParameters(sdIdConf, msg)

	addTraceParameters(msg, sdIdConf, record)

	msg.SetMessage(record.Body().AsString())

	msg.Valid()
	s, err := msg.String()
	if err != nil {
		return "", err
	}

	s = addNewLineIfNeeded(s)
	return s, nil
}

func addNewLineIfNeeded(s string) string {
	nl := ""
	if !strings.HasSuffix(s, "\n") {
		nl = "\n"
	}
	return s + nl
}

func addStandardFields(attributes pcommon.Map, msg *rfc5424.SyslogMessage) {
	if service, ok := attributes.Get("service.name"); ok {
		msg.SetAppname(service.AsString())
	}

	if hostname, ok := attributes.Get("host.name"); ok {
		msg.SetHostname(hostname.AsString())
	}

	if hostname, ok := attributes.Get("host.hostname"); ok {
		msg.SetHostname(hostname.AsString())
	}

	if procId, ok := attributes.Get("syslog.procid"); ok {
		msg.SetProcID(procId.AsString())
	}

	if msgId, ok := attributes.Get("syslog.msgid"); ok {
		msg.SetMsgID(msgId.AsString())
	}
}

func addAttributes(attributes pcommon.Map, sdIdConf SdConfig, msg *rfc5424.SyslogMessage) {
	//todo add doc that attributes will be merged
	//todo add to the doc that some attributes transferred to parameters

	attributes.Range(func(k string, v pcommon.Value) bool {
		// it will be added as app name, no needs in duplication
		excludedAttrs := []string{"service.name", "host.name", "host.hostname", "syslog.msgid", "syslog.procid"}
		for _, e := range excludedAttrs {
			if k == e {
				return true
			}
		}

		if mappedSdId, found := sdIdConf.CustomMapping[k]; found {
			msg.SetParameter(mappedSdId, k, v.AsString())
		} else {
			msg.SetParameter(sdIdConf.CommonSdId, k, v.AsString())
		}
		return true
	})
}

func mergeAttributes(resource pcommon.Resource, record plog.LogRecord) pcommon.Map {
	attributes := resource.Attributes()
	record.Attributes().Range(func(k string, v pcommon.Value) bool {
		attributes.Upsert(k, v)
		return true
	})
	return attributes
}

func addTraceParameters(msg *rfc5424.SyslogMessage, sdIdConf SdConfig, record plog.LogRecord) {

	if traceId := record.TraceID().HexString(); traceId != "" {
		msg.SetParameter(sdIdConf.TraceSdId, "trace_id", traceId)
	}

	if spanId := record.SpanID().HexString(); spanId != "" {
		msg.SetParameter(sdIdConf.TraceSdId, "span_id", spanId)
	}
}

func addStaticParameters(sdIdConf SdConfig, msg *rfc5424.SyslogMessage) {
	for staticSdid, params := range sdIdConf.StaticSd {
		for name, value := range params {
			msg.SetParameter(staticSdid, name, value)
		}
	}
}

func newExporter(logger *zap.Logger, cfg Config, client sysLogClient) (*syslogExporter, error) {
	cfg.setDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &syslogExporter{
		logger:      logger,
		client:      client,
		sdIdConf:    cfg.SdConfig,
		maxAttempts: 1,
	}, nil
}

func (e *syslogExporter) Shutdown(ctx context.Context) error {
	//todo support context
	return e.client.Close()
}

func (e *syslogExporter) Start(ctx context.Context, host component.Host) error {
	// no need to init
	return nil
}
