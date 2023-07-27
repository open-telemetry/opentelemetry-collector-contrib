package kineticaotelexporter

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/influxdata/influxdb-observability/common"
	"github.com/samber/lo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type kineticaLogsExporter struct {
	logger *zap.Logger

	writer *KiWriter
}

type kineticaLogRecord struct {
	log               *Log
	logAttribute      []LogAttribute
	resourceAttribute []ResourceAttribute
	scopeAttribute    []ScopeAttribute
}

var logTableDDLs = []string{
	CreateLog,
	CreateLogAttribute,
	CreateLogResourceAttribute,
	CreateLogScopeAttribute,
}

// newLogsExporter
//
//	@param logger
//	@param cfg
//	@return *kineticaLogsExporter
//	@return error
func newLogsExporter(logger *zap.Logger, cfg *Config) (*kineticaLogsExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	writer := NewKiWriter(context.TODO(), *cfg, logger)
	logsExp := &kineticaLogsExporter{
		logger: logger,
		writer: writer,
	}
	return logsExp, nil
}

func (e *kineticaLogsExporter) start(ctx context.Context, _ component.Host) error {

	// No schema specified in config
	fmt.Println("SCHEMA NAME - ", e.writer.cfg.Schema)

	if e.writer.cfg.Schema != "" && len(e.writer.cfg.Schema) != 0 {
		// Config has a schema name
		if err := createSchema(ctx, e.writer, e.writer.cfg); err != nil {
			return err
		}
	}
	if err := createLogTables(ctx, e.writer); err != nil {
		return err
	}
	return nil
}

// shutdown will shut down the exporter.
func (e *kineticaLogsExporter) shutdown(ctx context.Context) error {
	return nil
}

// createLogTables
//
//	@param ctx
//	@param kiWriter
//	@return error
func createLogTables(ctx context.Context, kiWriter *KiWriter) error {
	var errs []error

	var schema string
	schema = strings.Trim(kiWriter.cfg.Schema, " ")
	if len(schema) > 0 {
		schema = schema + "."
	} else {
		schema = ""
	}

	lo.ForEach(logTableDDLs, func(ddl string, index int) {

		stmt := strings.ReplaceAll(ddl, "%s", schema)
		kiWriter.logger.Debug("Creating Table - ", zap.String("DDL", stmt))

		_, err := kiWriter.Db.ExecuteSqlRaw(ctx, stmt, 0, 0, "", nil)
		if err != nil {
			kiWriter.logger.Error(err.Error())
			errs = append(errs, err)
		}
	})

	return multierr.Combine(errs...)
}

// pushLogsData
//
//	@receiver e
//	@param ctx
//	@param ld
//	@return error
func (e *kineticaLogsExporter) pushLogsData(ctx context.Context, logData plog.Logs) error {
	var errs []error
	var logRecords []kineticaLogRecord

	resourceLogs := logData.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		rl := resourceLogs.At(i)
		resource := rl.Resource()
		scopeLogs := rl.ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			scopeLog := scopeLogs.At(j)
			logs := scopeLogs.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				if kiLogRecord, err := e.createLogRecord(ctx, resource, scopeLog.Scope(), logs.At(k)); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}

					errs = append(errs, err)
				} else {
					logRecords = append(logRecords, *kiLogRecord)
				}
			}
		}
	}

	if err := e.writer.persistLogRecord(logRecords); err != nil {
		errs = append(errs, err)
	}
	return multierr.Combine(errs...)
}

// createLogRecord //
//
//	@receiver e
//	@param ctx
//	@param resource
//	@param instrumentationLibrary
//	@param logRecord
//	@return *kineticaLogRecord
//	@return error
func (e *kineticaLogsExporter) createLogRecord(ctx context.Context, resource pcommon.Resource, instrumentationLibrary pcommon.InstrumentationScope, logRecord plog.LogRecord) (*kineticaLogRecord, error) {
	var errs []error
	ts := logRecord.Timestamp().AsTime().UnixMilli()
	ots := logRecord.ObservedTimestamp().AsTime().UnixMilli()

	tags := make(map[string]string)
	fields := make(map[string]interface{})

	// TODO handle logRecord.Flags()

	if traceID := logRecord.TraceID(); !traceID.IsEmpty() {
		tags[AttributeTraceID] = hex.EncodeToString(traceID[:])
		if spanID := logRecord.SpanID(); !spanID.IsEmpty() {
			tags[AttributeSpanID] = hex.EncodeToString(spanID[:])
		}
	}

	if severityNumber := logRecord.SeverityNumber(); severityNumber != plog.SeverityNumberUnspecified {
		fields[AttributeSeverityNumber] = int64(severityNumber)
	}
	if severityText := logRecord.SeverityText(); severityText != "" {
		fields[AttributeSeverityText] = severityText
	}

	if v, err := AttributeValueToKineticaFieldValue(logRecord.Body()); err != nil {
		e.logger.Debug("Invalid log record body", zap.String("Error", err.Error()))
		fields[AttributeBody] = nil
	} else {
		fields[AttributeBody] = v
	}

	var logAttribute []LogAttribute
	logAttributes := make(map[string]ValueTypePair)
	droppedAttributesCount := uint64(logRecord.DroppedAttributesCount())
	logRecord.Attributes().Range(func(k string, v pcommon.Value) bool {
		if k == "" {
			droppedAttributesCount++
			e.logger.Debug("Log record attribute key is empty")
		} else if v, err := AttributeValueToKineticaFieldValue(v); err != nil {
			droppedAttributesCount++
			e.logger.Debug("Invalid log record attribute value", zap.String("Error", err.Error()))
		} else {
			logAttributes[k] = v
		}
		return true
	})

	// fmt.Println("Received Log Attributes - ", logAttributes)

	if droppedAttributesCount > 0 {
		fields[common.AttributeDroppedAttributesCount] = droppedAttributesCount
	}

	severityNumber, ok := (fields[AttributeSeverityNumber]).(int)
	if !ok {
		severityNumber = 0
		e.logger.Warn("Severity number conversion failed, possibly not an integer; storing 0")
	}

	severityText, ok := (fields[AttributeSeverityText]).(string)
	if !ok {
		e.logger.Warn("Severity text conversion failed, possibly not a string; storing empty string")
		severityText = ""
	}

	// create log - Body, dropped_attribute_count and flags not handled now
	log := NewLog(uuid.New().String(), tags[AttributeTraceID], tags[AttributeSpanID], ts, ots, int8(severityNumber), severityText, fields[AttributeBody].(ValueTypePair).value.(string), 0)
	// _, err := log.insertLog()
	// errs = append(errs, err)

	// Insert log attributes
	for key := range logAttributes {
		vtPair := logAttributes[key]

		// fmt.Println("Log Attibute Value Type pair - ", vtPair)

		la, err := newLogAttributeValue(log.LogID, key, vtPair)

		// fmt.Printf("Log Attribute Value = %#v\n", la)
		if err == nil {
			logAttribute = append(logAttribute, *la)
		} else {
			e.logger.Error(err.Error())
		}
	}

	var resourceAttribute []ResourceAttribute
	// Insert resource attributes
	resourceAttributes := make(map[string]ValueTypePair)
	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		if k == "" {
			e.logger.Debug("Resource attribute key is empty")
		} else if v, err := AttributeValueToKineticaFieldValue(v); err != nil {
			e.logger.Debug("Invalid resource attribute value", zap.String("Error", err.Error()))
		} else {
			resourceAttributes[k] = v
		}
		return true
	})

	for key := range resourceAttributes {
		vtPair := resourceAttributes[key]
		ra, err := newResourceAttributeValue(log.LogID, key, vtPair)
		if err == nil {
			resourceAttribute = append(resourceAttribute, *ra)
		} else {
			e.logger.Error(err.Error())
		}
	}

	// Insert scope attributes
	var scopeAttribute []ScopeAttribute
	scopeAttributes := make(map[string]ValueTypePair)
	scopeName := instrumentationLibrary.Name()
	scopeVersion := instrumentationLibrary.Version()
	instrumentationLibrary.Attributes().Range(func(k string, v pcommon.Value) bool {
		if k == "" {
			e.logger.Debug("Scope attribute key is empty")
		} else if v, err := AttributeValueToKineticaFieldValue(v); err != nil {
			e.logger.Debug("Invalid scope attribute value", zap.String("Error", err.Error()))
		} else {
			scopeAttributes[k] = v
		}
		return true
	})

	for key := range scopeAttributes {
		vtPair := scopeAttributes[key]
		sa := newScopeAttributeValue(log.LogID, key, scopeName, scopeVersion, vtPair)
		scopeAttribute = append(scopeAttribute, *sa)
	}

	// fmt.Printf("Log attribute %#v\n", logAttribute)
	// fmt.Printf("Resource Attribute %#v\n", resourceAttribute)
	// fmt.Printf("Scope Attribute %#v\n", scopeAttribute)

	kiLogRecord := new(kineticaLogRecord)
	kiLogRecord.log = log

	kiLogRecord.logAttribute = make([]LogAttribute, len(logAttribute))
	copy(kiLogRecord.logAttribute, logAttribute)

	kiLogRecord.resourceAttribute = make([]ResourceAttribute, len(resourceAttribute))
	copy(kiLogRecord.resourceAttribute, resourceAttribute)

	kiLogRecord.scopeAttribute = make([]ScopeAttribute, len(scopeAttribute))
	copy(kiLogRecord.scopeAttribute, scopeAttribute)

	finalLogRecrodStr := fmt.Sprintf("Final Log Record %#v\n", kiLogRecord)
	e.logger.Debug(finalLogRecrodStr)

	return kiLogRecord, multierr.Combine(errs...)

}

// newLogAttributeValue
//
//	@param logID
//	@param key
//	@param vtPair
//	@return *LogsAttribute
func newLogAttributeValue(logID string, key string, vtPair ValueTypePair) (*LogAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)

	if err == nil {
		la := NewLogAttribute(logID, key, *av)
		return la, nil
	}

	return nil, err
}

// newResourceAttributeValue
//
//	@param resourceID
//	@param key
//	@param vtPair
//	@return *LogsResourceAttribute
func newResourceAttributeValue(resourceID string, key string, vtPair ValueTypePair) (*ResourceAttribute, error) {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)

	if err == nil {
		la := NewResourceAttribute(resourceID, key, *av)
		return la, nil
	}

	return nil, err
}

// newScopeAttributeValue newResourceAttributeValue
//
//	@param key
//	@param scopeName
//	@param scopeVersion
//	@param vtPair
//	@return *LogsScopeAttribute
func newScopeAttributeValue(scopeID string, key string, scopeName string, scopeVersion string, vtPair ValueTypePair) *ScopeAttribute {
	var av *AttributeValue
	var err error

	av, err = getAttributeValue(vtPair)

	if err != nil {
		sa := NewScopeAttribute(scopeID, key, scopeName, scopeVersion, *av)
		return sa
	}

	return nil
}
