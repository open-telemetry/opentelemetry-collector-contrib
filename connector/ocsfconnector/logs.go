// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ocsfconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/ocsfconnector"

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"text/template"
	"time"

	"github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

const (
	vendorName  = "OpenTelemetry"
	productName = "ocsfconnector"
	version     = "1"
)

type ocsfProduct struct {
	VendorName string `json:"vendor_name"`
	Name       string `json:"name"`
}

type ocsfMetadata struct {
	Version string      `json:"version"`
	Product ocsfProduct `json:"product"`
}

type ocsfEvent struct {
	Metadata     ocsfMetadata `json:"metadata"`
	Time         int64        `json:"time"`
	ClassUID     int          `json:"class_uid"`
	ClassName    string       `json:"class_name"`
	CategoryUID  int          `json:"category_uid"`
	CategoryName string       `json:"category_name"`
	ActivityID   int          `json:"activity_id"`
	ActivityName string       `json:"activity_name"`
	SeverityID   int          `json:"severity_id"`
	Severity     string       `json:"severity"`
	Message      string       `json:"message"`
}

type connectorLogs struct {
	config       Config
	logsConsumer consumer.Logs
	logger       *zap.Logger

	component.StartFunc
	component.ShutdownFunc
	cache []mappingCache
}

type mappingCache struct {
	detection *regexp.Regexp
	template  *template.Template
}

// newLogsConnector is a function to create a new connector for logs extraction
func newLogsConnector(set connector.Settings, config component.Config, logsConsumer consumer.Logs) (*connectorLogs, error) {
	cfg := config.(*Config)
	mappingCaches := make([]mappingCache, len(cfg.Mappings))

	var errs []error
	for i, m := range cfg.Mappings {
		d, err := regexp.Compile(m.Detection)
		errs = append(errs, err)
		tmpl, err := template.New(m.ActivityName).Parse(m.MessageTemplate)
		errs = append(errs, err)
		mappingCaches[i] = mappingCache{
			detection: d,
			template:  tmpl,
		}
	}

	return &connectorLogs{
		config:       *cfg,
		logger:       set.Logger,
		logsConsumer: logsConsumer,
		cache:        mappingCaches,
	}, errors.Join(errs...)
}

// Capabilities implements the consumer interface
func (*connectorLogs) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs method is called for each instance of a log sent to the connector
func (c *connectorLogs) ConsumeLogs(ctx context.Context, pl plog.Logs) error {
	result := plog.NewLogs()
	resultScopelogs := result.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()

	var errs []error
	for i := 0; i < pl.ResourceLogs().Len(); i++ {
		li := pl.ResourceLogs().At(i)
		for j := 0; j < li.ScopeLogs().Len(); j++ {
			sl := li.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				strBody := lr.Body().Str()
				for i, m := range c.config.Mappings {
					regex := c.cache[i].detection
					if detected := regex.FindStringSubmatch(strBody); detected != nil {
						names := regex.SubexpNames()
						capturedGroups := make(map[string]string, len(names))
						for i, name := range names {
							if i != 0 && name != "" {
								capturedGroups[name] = detected[i]
							}
						}
						output := bytes.NewBuffer(make([]byte, 0, 1024))
						err := c.cache[i].template.Execute(output, capturedGroups)
						if err != nil {
							errs = append(errs, err)
						} else {
							newEvent := ocsfEvent{
								Metadata: ocsfMetadata{
									Version: version,
									Product: ocsfProduct{
										VendorName: vendorName,
										Name:       productName,
									},
								},
								Time:         time.Now().UnixNano(),
								ClassUID:     m.ClassUID,
								ClassName:    m.ClassName,
								CategoryUID:  m.CategoryUID,
								CategoryName: m.CategoryName,
								ActivityID:   m.ActivityID,
								ActivityName: m.ActivityName,
								SeverityID:   m.SeverityID,
								Severity:     m.Severity,
								Message:      output.String(),
							}
							b, err := json.Marshal(newEvent)
							if err != nil {
								errs = append(errs, err)
							} else {
								ocsfLogRecord := resultScopelogs.LogRecords().AppendEmpty()
								ocsfLogRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
								ocsfLogRecord.Body().SetStr(string(b))
							}
						}
					}
				}
			}
		}
	}
	errs = append(errs, c.logsConsumer.ConsumeLogs(ctx, result))
	return errors.Join(errs...)
}
