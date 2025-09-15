// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoracledbreceiver

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// LogsReceiver implements the logs receiver for Oracle DB
type LogsReceiver struct {
	cfg          *Config
	consumer     consumer.Logs
	logger       *zap.Logger
	dbClient     dbClient
	cancel       context.CancelFunc
	ticker       *time.Ticker
	lastPosition map[string]int64 // Track file positions
}

// Oracle log patterns
var (
	// Oracle alert log pattern: YYYY-MM-DD HH:MM:SS.SSS +TZ:TZ
	alertLogTimestampPattern = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}[+-]\d{2}:\d{2})`)

	// Oracle error pattern: ORA-##### or TNS-##### or IMP-##### etc.
	oracleErrorPattern = regexp.MustCompile(`([A-Z]{3}-\d{5}): (.*)`)

	// Trace file timestamp pattern
	traceTimestampPattern = regexp.MustCompile(`^(\w{3}\s+\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\s+\d{4})`)

	// SQL statement pattern in logs
	sqlStatementPattern = regexp.MustCompile(`(?i)(SELECT|INSERT|UPDATE|DELETE|CREATE|DROP|ALTER|GRANT|REVOKE)\s+.*`)
)

// Oracle log severities mapping
var oracleSeverityMap = map[string]plog.SeverityNumber{
	"TRACE":   plog.SeverityNumberTrace,
	"DEBUG":   plog.SeverityNumberDebug,
	"INFO":    plog.SeverityNumberInfo,
	"WARNING": plog.SeverityNumberWarn,
	"ERROR":   plog.SeverityNumberError,
	"FATAL":   plog.SeverityNumberFatal,
	"UNKNOWN": plog.SeverityNumberUnspecified,
}

// NewLogsReceiver creates a new Oracle logs receiver
func NewLogsReceiver(cfg *Config, settings receiver.Settings, consumer consumer.Logs) (*LogsReceiver, error) {
	if !cfg.LogsConfig.EnableLogs {
		return nil, errors.New("logs collection is not enabled")
	}

	return &LogsReceiver{
		cfg:          cfg,
		consumer:     consumer,
		logger:       settings.Logger,
		lastPosition: make(map[string]int64),
	}, nil
}

// Start starts the logs receiver
func (lr *LogsReceiver) Start(ctx context.Context, _ component.Host) error {
	lr.logger.Info("Starting Oracle logs receiver")

	// Create database client if query-based collection is enabled
	if lr.cfg.LogsConfig.QueryBasedCollection {
		client, err := newDBClient(lr.cfg, lr.logger)
		if err != nil {
			return fmt.Errorf("failed to create database client for logs: %w", err)
		}
		lr.dbClient = client
	}

	// Parse poll interval
	interval, err := time.ParseDuration(lr.cfg.LogsConfig.PollInterval)
	if err != nil {
		interval = 30 * time.Second // Default fallback
		lr.logger.Warn("Invalid poll interval, using default", zap.String("interval", interval.String()))
	}

	// Create context and ticker
	ctx, lr.cancel = context.WithCancel(ctx)
	lr.ticker = time.NewTicker(interval)

	// Start collection in background
	go lr.collectLogs(ctx)

	return nil
}

// Shutdown stops the logs receiver
func (lr *LogsReceiver) Shutdown(_ context.Context) error {
	lr.logger.Info("Shutting down Oracle logs receiver")

	if lr.cancel != nil {
		lr.cancel()
	}

	if lr.ticker != nil {
		lr.ticker.Stop()
	}

	if lr.dbClient != nil {
		lr.dbClient.close()
	}

	return nil
}

// collectLogs is the main collection loop
func (lr *LogsReceiver) collectLogs(ctx context.Context) {
	lr.logger.Info("Started Oracle logs collection")

	// Initial collection
	lr.performCollection(ctx)

	// Periodic collection
	for {
		select {
		case <-ctx.Done():
			lr.logger.Info("Oracle logs collection stopped")
			return
		case <-lr.ticker.C:
			lr.performCollection(ctx)
		}
	}
}

// performCollection performs one round of log collection
func (lr *LogsReceiver) performCollection(ctx context.Context) {
	logs := plog.NewLogs()

	// Collect from different sources
	if lr.cfg.LogsConfig.CollectAlertLogs {
		lr.collectAlertLogs(ctx, logs)
	}

	if lr.cfg.LogsConfig.CollectAuditLogs {
		lr.collectAuditLogs(ctx, logs)
	}

	if lr.cfg.LogsConfig.CollectTraceFiles {
		lr.collectTraceFiles(ctx, logs)
	}

	if lr.cfg.LogsConfig.CollectArchiveLogs {
		lr.collectArchiveLogs(ctx, logs)
	}

	// Query-based collection
	if lr.cfg.LogsConfig.QueryBasedCollection {
		lr.collectQueryBasedLogs(ctx, logs)
	}

	// Send logs if we have any
	if logs.LogRecordCount() > 0 {
		if err := lr.consumer.ConsumeLogs(ctx, logs); err != nil {
			lr.logger.Error("Failed to send logs", zap.Error(err))
		} else {
			lr.logger.Debug("Successfully sent logs", zap.Int("count", logs.LogRecordCount()))
		}
	}
}

// collectAlertLogs collects Oracle alert logs
func (lr *LogsReceiver) collectAlertLogs(ctx context.Context, logs plog.Logs) {
	if lr.cfg.LogsConfig.AlertLogPath == "" {
		return
	}

	files, err := filepath.Glob(lr.cfg.LogsConfig.AlertLogPath)
	if err != nil {
		lr.logger.Error("Failed to find alert log files", zap.Error(err), zap.String("pattern", lr.cfg.LogsConfig.AlertLogPath))
		return
	}

	for _, file := range files {
		lr.collectFromFile(ctx, file, "alert", logs, lr.parseAlertLogEntry)
	}
}

// collectAuditLogs collects Oracle audit logs
func (lr *LogsReceiver) collectAuditLogs(ctx context.Context, logs plog.Logs) {
	if lr.cfg.LogsConfig.AuditLogPath == "" {
		return
	}

	// Audit logs are typically in a directory
	files, err := filepath.Glob(filepath.Join(lr.cfg.LogsConfig.AuditLogPath, "*.aud"))
	if err != nil {
		lr.logger.Error("Failed to find audit log files", zap.Error(err), zap.String("path", lr.cfg.LogsConfig.AuditLogPath))
		return
	}

	for _, file := range files {
		lr.collectFromFile(ctx, file, "audit", logs, lr.parseAuditLogEntry)
	}
}

// collectTraceFiles collects Oracle trace files
func (lr *LogsReceiver) collectTraceFiles(ctx context.Context, logs plog.Logs) {
	if lr.cfg.LogsConfig.TraceLogPath == "" {
		return
	}

	files, err := filepath.Glob(filepath.Join(lr.cfg.LogsConfig.TraceLogPath, "*.trc"))
	if err != nil {
		lr.logger.Error("Failed to find trace files", zap.Error(err), zap.String("path", lr.cfg.LogsConfig.TraceLogPath))
		return
	}

	for _, file := range files {
		lr.collectFromFile(ctx, file, "trace", logs, lr.parseTraceLogEntry)
	}
}

// collectArchiveLogs collects Oracle archive logs (metadata)
func (lr *LogsReceiver) collectArchiveLogs(ctx context.Context, logs plog.Logs) {
	if lr.cfg.LogsConfig.ArchiveLogPath == "" {
		return
	}

	files, err := filepath.Glob(filepath.Join(lr.cfg.LogsConfig.ArchiveLogPath, "*.arc"))
	if err != nil {
		lr.logger.Error("Failed to find archive log files", zap.Error(err), zap.String("path", lr.cfg.LogsConfig.ArchiveLogPath))
		return
	}

	for _, file := range files {
		lr.collectArchiveLogMetadata(ctx, file, logs)
	}
}

// collectFromFile collects logs from a specific file
func (lr *LogsReceiver) collectFromFile(ctx context.Context, filePath string, logType string, logs plog.Logs, parser func(string) *LogEntry) {
	file, err := os.Open(filePath)
	if err != nil {
		lr.logger.Error("Failed to open log file", zap.Error(err), zap.String("file", filePath))
		return
	}
	defer file.Close()

	// Get file info for size check
	info, err := file.Stat()
	if err != nil {
		lr.logger.Error("Failed to get file info", zap.Error(err), zap.String("file", filePath))
		return
	}

	// Check file size limit
	if lr.cfg.LogsConfig.MaxLogFileSize > 0 && info.Size() > lr.cfg.LogsConfig.MaxLogFileSize*1024*1024 {
		lr.logger.Warn("Skipping large log file", zap.String("file", filePath), zap.Int64("size_mb", info.Size()/1024/1024))
		return
	}

	// Seek to last known position
	lastPos := lr.lastPosition[filePath]
	if lastPos > 0 && lastPos < info.Size() {
		if _, err := file.Seek(lastPos, 0); err != nil {
			lr.logger.Warn("Failed to seek file position", zap.Error(err), zap.String("file", filePath))
		}
	}

	// Read new content
	scanner := bufio.NewScanner(file)
	lineCount := 0
	entriesAdded := 0

	resourceLogs := logs.ResourceLogs().AppendEmpty()
	lr.setResourceAttributes(resourceLogs.Resource(), filePath, logType)
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

	for scanner.Scan() && entriesAdded < lr.cfg.LogsConfig.BatchSize {
		line := scanner.Text()
		lineCount++

		// Parse log entry
		entry := parser(line)
		if entry == nil {
			continue
		}

		// Apply filters
		if !lr.shouldIncludeLogEntry(entry) {
			continue
		}

		// Create log record
		logRecord := scopeLogs.LogRecords().AppendEmpty()
		lr.populateLogRecord(logRecord, entry, filePath, logType)
		entriesAdded++

		select {
		case <-ctx.Done():
			return
		default:
		}
	}

	// Update position
	newPos, _ := file.Seek(0, io.SeekCurrent)
	lr.lastPosition[filePath] = newPos

	lr.logger.Debug("Processed log file",
		zap.String("file", filePath),
		zap.String("type", logType),
		zap.Int("lines_read", lineCount),
		zap.Int("entries_added", entriesAdded))
}

// LogEntry represents a parsed log entry
type LogEntry struct {
	Timestamp    time.Time
	Level        string
	Message      string
	Component    string
	SessionID    string
	ProcessID    string
	ThreadID     string
	UserID       string
	ClientHost   string
	Program      string
	ErrorCode    string
	SQLStatement string
	Operation    string
	ObjectName   string
	AuditAction  string
	AuditResult  string
	RawLine      string
}

// parseAlertLogEntry parses Oracle alert log entries
func (lr *LogsReceiver) parseAlertLogEntry(line string) *LogEntry {
	if strings.TrimSpace(line) == "" {
		return nil
	}

	entry := &LogEntry{
		RawLine: line,
		Level:   "INFO", // Default level for alert logs
	}

	// Try to parse timestamp
	if matches := alertLogTimestampPattern.FindStringSubmatch(line); len(matches) > 1 {
		if ts, err := time.Parse("2006-01-02T15:04:05.000-07:00", matches[1]); err == nil {
			entry.Timestamp = ts
			entry.Message = strings.TrimSpace(line[len(matches[0]):])
		} else {
			entry.Timestamp = time.Now()
			entry.Message = line
		}
	} else {
		entry.Timestamp = time.Now()
		entry.Message = line
	}

	// Parse Oracle error codes
	if matches := oracleErrorPattern.FindStringSubmatch(entry.Message); len(matches) > 2 {
		entry.ErrorCode = matches[1]
		entry.Level = "ERROR"
		entry.Message = matches[2]
	}

	// Extract SQL statements if enabled
	if lr.cfg.LogsConfig.ParseSQLStatements {
		if matches := sqlStatementPattern.FindStringSubmatch(entry.Message); len(matches) > 0 {
			entry.SQLStatement = strings.TrimSpace(matches[0])
			entry.Operation = strings.ToUpper(strings.Fields(entry.SQLStatement)[0])
		}
	}

	return entry
}

// parseAuditLogEntry parses Oracle audit log entries
func (*LogsReceiver) parseAuditLogEntry(line string) *LogEntry {
	if strings.TrimSpace(line) == "" {
		return nil
	}

	entry := &LogEntry{
		RawLine: line,
		Level:   "INFO",
	}

	// Basic audit log parsing (XML format common in newer Oracle versions)
	if strings.Contains(line, "<Audit") {
		entry.Message = line
		entry.Component = "audit"

		// Extract audit action
		if strings.Contains(line, "ACTION") {
			// Simple regex for ACTION attribute
			if re := regexp.MustCompile(`ACTION="([^"]+)"`); re != nil {
				if matches := re.FindStringSubmatch(line); len(matches) > 1 {
					entry.AuditAction = matches[1]
				}
			}
		}

		// Extract result
		if strings.Contains(line, "RETURNCODE") {
			if re := regexp.MustCompile(`RETURNCODE="([^"]+)"`); re != nil {
				if matches := re.FindStringSubmatch(line); len(matches) > 1 {
					entry.AuditResult = matches[1]
					if matches[1] != "0" {
						entry.Level = "ERROR"
					}
				}
			}
		}

		entry.Timestamp = time.Now() // Could parse from audit record
	} else {
		// Legacy audit format
		entry.Message = line
		entry.Timestamp = time.Now()
	}

	return entry
}

// parseTraceLogEntry parses Oracle trace file entries
func (*LogsReceiver) parseTraceLogEntry(line string) *LogEntry {
	if strings.TrimSpace(line) == "" {
		return nil
	}

	entry := &LogEntry{
		RawLine: line,
		Level:   "DEBUG", // Trace files are typically debug level
	}

	// Try to parse trace timestamp
	if matches := traceTimestampPattern.FindStringSubmatch(line); len(matches) > 1 {
		if ts, err := time.Parse("Mon Jan 2 15:04:05 2006", matches[1]); err == nil {
			entry.Timestamp = ts
			entry.Message = strings.TrimSpace(line[len(matches[0]):])
		} else {
			entry.Timestamp = time.Now()
			entry.Message = line
		}
	} else {
		entry.Timestamp = time.Now()
		entry.Message = line
	}

	// Parse trace-specific information
	if strings.Contains(line, "*** SESSION ID:") {
		if re := regexp.MustCompile(`SESSION ID:\((\d+\.\d+)\)`); re != nil {
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				entry.SessionID = matches[1]
			}
		}
	}

	return entry
}

// collectArchiveLogMetadata collects metadata about archive logs
func (lr *LogsReceiver) collectArchiveLogMetadata(_ context.Context, filePath string, logs plog.Logs) {
	info, err := os.Stat(filePath)
	if err != nil {
		return
	}

	resourceLogs := logs.ResourceLogs().AppendEmpty()
	lr.setResourceAttributes(resourceLogs.Resource(), filePath, "archive")

	logRecord := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(info.ModTime()))
	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
	logRecord.SetSeverityText("INFO")

	logRecord.Body().SetStr(fmt.Sprintf("Archive log file: %s, size: %d bytes", filepath.Base(filePath), info.Size()))

	attrs := logRecord.Attributes()
	attrs.PutStr("log.file.name", filepath.Base(filePath))
	attrs.PutStr("log.file.path", filePath)
	attrs.PutInt("log.file.size", info.Size())
	attrs.PutStr("log.component", "archive")
}

// collectQueryBasedLogs collects logs using SQL queries
func (lr *LogsReceiver) collectQueryBasedLogs(ctx context.Context, logs plog.Logs) {
	if lr.dbClient == nil {
		return
	}

	// Collect alert logs via query
	if lr.cfg.LogsConfig.AlertLogQuery != "" {
		lr.executeLogQuery(ctx, lr.cfg.LogsConfig.AlertLogQuery, "alert", logs)
	}

	// Collect audit logs via query
	if lr.cfg.LogsConfig.AuditLogQuery != "" {
		lr.executeLogQuery(ctx, lr.cfg.LogsConfig.AuditLogQuery, "audit", logs)
	}

	// Default queries if none specified
	if lr.cfg.LogsConfig.AlertLogQuery == "" {
		// Query V$DIAG_ALERT_EXT for alert log entries
		defaultAlertQuery := `
			SELECT message_text, originating_timestamp, component_id, host_id, host_address
			FROM v$diag_alert_ext 
			WHERE originating_timestamp > SYSTIMESTAMP - INTERVAL '1' HOUR
			ORDER BY originating_timestamp DESC`
		lr.executeLogQuery(ctx, defaultAlertQuery, "alert_query", logs)
	}
}

// executeLogQuery executes a SQL query to collect logs
func (lr *LogsReceiver) executeLogQuery(ctx context.Context, query string, logType string, logs plog.Logs) {
	// Get database connection
	db, err := lr.dbClient.getConnection()
	if err != nil {
		lr.logger.Error("Failed to get database connection for log query", zap.Error(err), zap.String("type", logType))
		return
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		lr.logger.Error("Failed to execute log query", zap.Error(err), zap.String("type", logType))
		return
	}
	defer rows.Close()

	resourceLogs := logs.ResourceLogs().AppendEmpty()
	lr.setResourceAttributes(resourceLogs.Resource(), "", logType)
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

	count := 0
	for rows.Next() && count < lr.cfg.LogsConfig.BatchSize {
		var message, component, hostID, hostAddr sql.NullString
		var timestamp sql.NullTime

		if err := rows.Scan(&message, &timestamp, &component, &hostID, &hostAddr); err != nil {
			lr.logger.Error("Failed to scan log query result", zap.Error(err))
			continue
		}

		logRecord := scopeLogs.LogRecords().AppendEmpty()

		// Set timestamp
		if timestamp.Valid {
			logRecord.SetTimestamp(pcommon.NewTimestampFromTime(timestamp.Time))
		} else {
			logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		}

		logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
		logRecord.SetSeverityText("INFO")

		// Set message
		if message.Valid {
			logRecord.Body().SetStr(message.String)
		}

		// Set attributes
		attrs := logRecord.Attributes()
		attrs.PutStr("log.source", "query")
		attrs.PutStr("log.message.type", logType)

		if component.Valid {
			attrs.PutStr("log.component", component.String)
		}
		if hostID.Valid {
			attrs.PutStr("log.host_id", hostID.String)
		}
		if hostAddr.Valid {
			attrs.PutStr("log.host_address", hostAddr.String)
		}

		count++
	}

	lr.logger.Debug("Collected logs via query", zap.String("type", logType), zap.Int("count", count))
}

// shouldIncludeLogEntry determines if a log entry should be included based on filters
func (lr *LogsReceiver) shouldIncludeLogEntry(entry *LogEntry) bool {
	// Check minimum log level
	if lr.cfg.LogsConfig.MinLogLevel != "" {
		entryLevel := oracleSeverityMap[strings.ToUpper(entry.Level)]
		minLevel := oracleSeverityMap[strings.ToUpper(lr.cfg.LogsConfig.MinLogLevel)]
		if entryLevel < minLevel {
			return false
		}
	}

	// Check exclude levels
	for _, excludeLevel := range lr.cfg.LogsConfig.ExcludeLevels {
		if strings.EqualFold(entry.Level, excludeLevel) {
			return false
		}
	}

	// Check include/exclude classes (components)
	if len(lr.cfg.LogsConfig.IncludeClasses) > 0 {
		found := false
		for _, includeClass := range lr.cfg.LogsConfig.IncludeClasses {
			if strings.Contains(strings.ToLower(entry.Component), strings.ToLower(includeClass)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	for _, excludeClass := range lr.cfg.LogsConfig.ExcludeClasses {
		if strings.Contains(strings.ToLower(entry.Component), strings.ToLower(excludeClass)) {
			return false
		}
	}

	return true
}

// setResourceAttributes sets resource attributes for log records
func (lr *LogsReceiver) setResourceAttributes(resource pcommon.Resource, filePath string, logType string) {
	attrs := resource.Attributes()

	// Set standard attributes
	if lr.cfg.GetEffectiveEndpoint() != "" {
		attrs.PutStr("host.name", strings.Split(lr.cfg.GetEffectiveEndpoint(), ":")[0])
	}

	attrs.PutStr("oracledb.log.source", logType)

	if filePath != "" {
		attrs.PutStr("oracledb.log.file.name", filepath.Base(filePath))
		attrs.PutStr("oracledb.log.file.path", filePath)
	}

	if lr.cfg.GetEffectiveService() != "" {
		attrs.PutStr("oracledb.database.sid", lr.cfg.GetEffectiveService())
	}

	attrs.PutStr("oracledb.instance.name", lr.cfg.GetEffectiveService())
}

// populateLogRecord populates a log record with parsed log entry data
func (*LogsReceiver) populateLogRecord(logRecord plog.LogRecord, entry *LogEntry, _ string, logType string) {
	// Set timestamps
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(entry.Timestamp))
	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Set severity
	if severity, ok := oracleSeverityMap[strings.ToUpper(entry.Level)]; ok {
		logRecord.SetSeverityNumber(severity)
	} else {
		logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
	}
	logRecord.SetSeverityText(strings.ToUpper(entry.Level))

	// Set body
	logRecord.Body().SetStr(entry.Message)

	// Set attributes
	attrs := logRecord.Attributes()
	attrs.PutStr("log.severity", entry.Level)
	attrs.PutStr("log.message.type", logType)

	if entry.Component != "" {
		attrs.PutStr("log.component", entry.Component)
	}
	if entry.SessionID != "" {
		attrs.PutStr("log.session_id", entry.SessionID)
	}
	if entry.ProcessID != "" {
		attrs.PutStr("log.process_id", entry.ProcessID)
	}
	if entry.ThreadID != "" {
		attrs.PutStr("log.thread_id", entry.ThreadID)
	}
	if entry.UserID != "" {
		attrs.PutStr("log.user_id", entry.UserID)
	}
	if entry.ClientHost != "" {
		attrs.PutStr("log.client_host", entry.ClientHost)
	}
	if entry.Program != "" {
		attrs.PutStr("log.client_program", entry.Program)
	}
	if entry.ErrorCode != "" {
		attrs.PutStr("log.error_code", entry.ErrorCode)
	}
	if entry.SQLStatement != "" {
		attrs.PutStr("log.sql_statement", entry.SQLStatement)
	}
	if entry.Operation != "" {
		attrs.PutStr("log.operation", entry.Operation)
	}
	if entry.ObjectName != "" {
		attrs.PutStr("log.object_name", entry.ObjectName)
	}
	if entry.AuditAction != "" {
		attrs.PutStr("log.audit.action", entry.AuditAction)
	}
	if entry.AuditResult != "" {
		attrs.PutStr("log.audit.result", entry.AuditResult)
	}
}
